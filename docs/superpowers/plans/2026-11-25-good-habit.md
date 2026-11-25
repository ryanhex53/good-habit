# 好习惯（good-habit）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现家庭积分 + 转盘抽奖系统全栈：家长登记得分、孩子转盘抽奖（单抽/保险双抽）、积分账单；Go 单二进制 + SQLite 读写分离 + 内存热路径 + 异步组提交 + Litestream 容灾 + Vue 3 前端。

**Architecture:** 单进程 Go 服务：ReadDB（`mode=ro`+`query_only`，5 连接）服务查询；WriteDB（`_txlock=immediate`，1 连接 + `WriteMu`）承载全部写入；孩子余额/家庭配置快照/会话/限流/今日计数常驻内存（细粒度锁 + atomic）；写入事件进 8192 channel，单写协程按 64 条或 15ms 组提交落盘；优雅停机 Drain。家长 CRUD 走同步写路径后原子替换内存快照。Vue 3 + Vite 构建到 `internal/webui/dist`，与 `migrations/*.sql` 一起 embed。

**Tech Stack:** Go ≥1.23（外部依赖仅 modernc.org/sqlite + golang.org/x/crypto/bcrypt）、net/http 标准库方法路由、Vue 3 + TypeScript + vue-router + Vite、systemd、Litestream（S3 → Cloudflare R2）。

**Spec:** `docs/superpowers/specs/2026-11-25-good-habit-design.md` —— 执行者须先通读设计文档；DDL 取 §7、PRAGMA 与池参数取 §6.2、API 形状取 §8、种子数据取附录 A、丢失窗口声明取 §9。

## Global Constraints

- module `good-habit`；后端外部依赖**仅** `modernc.org/sqlite` 与 `golang.org/x/crypto`（bcrypt cost 10）；`CGO_ENABLED=0 go build ./...` 必须始终成功。
- 前端依赖**仅** `vue`、`vue-router`（dev：`vite`、`@vitejs/plugin-vue`、`typescript`、`vue-tsc`）；构建产物输出 `internal/webui/dist`；不引 Pinia / UI 组件库 / CSS 框架。
- 写池 DSN：`file:<db>?_txlock=immediate&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=mmap_size(134217728)&_pragma=cache_size(-32000)&_pragma=wal_autocheckpoint(0)`，`SetMaxOpenConns(1)`；读池 DSN：`file:<db>?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)&_pragma=mmap_size(134217728)&_pragma=cache_size(-32000)`，连接数默认 5（区间 4–6）。若钉住的 modernc 版本对个别 `_pragma` 拼法不接受，按其文档调整拼写（语义不变）——以 Task 2 的测试为唯一裁判。
- Writer：`chan ledger.Op` 容量 8192；批 64 条 / 15ms；`Stop()` 必须排空队列后返回。
- 余额永不为负；所有 SQL 强制 `family_id` 过滤；`DELETE` 一律软删除（`active=0`）；时间一律 Unix 毫秒 int64；UI 文案简体中文。
- 用户名 3–32 位 `[a-z0-9_]` 小写存储；密码 ≥ 8；Cookie `gh_session`：HttpOnly、SameSite=Strict、Path=/、30 天，`Secure` 由 `GOODHABIT_SECURE_COOKIE=1` 开启。
- Go 测试一律用 `t.TempDir()` 真实 SQLite 文件（禁止 mock 数据库）；后端任务提交前 `go test ./... -race` 全绿；前端任务提交前 `cd web && npm run build` 通过。
- 提交信息用 Conventional Commits（feat/test/chore/docs），结尾加 `Co-Authored-By: Claude Code <noreply@anthropic.com>`。

## Review Focus

实现中最容易坑到真实用户、且已由具体任务测试钉住的五类输入/失效模式：

1. 同一孩子并发连点抽奖把余额抽负 → Task 15：10 goroutine 抢 5 份额度的 `-race` 并发测试。
2. 奖品池为空时抽奖 → Task 15：`no_active_prizes` 409 且账单行数、余额零变化。
3. 双抽未选完就再抽 → Task 15：`pending_choice_exists` 409；超时清扫测试：48h 后自动保留 A。
4. 跨家庭越权读写 → Task 11/16：B 家访问 A 家孩子/账单/抽奖一律 404/空集。
5. 登录爆破与全局滥用 → Task 7：5 次失败锁 10 分钟；Task 8：限流窗口与 rollover。

---

### Task 1: 项目骨架与配置加载

**Files:**
- Create: `go.mod`、`cmd/good-habit/main.go`、`internal/config/config.go`、`internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config{Addr, DBPath, DayTZ string; TrustProxy, SecureCookie bool; ReadPool int}`；`config.Load(args []string) (Config, error)`（flags `-addr`、`-db`；env `GOODHABIT_ADDR`、`GOODHABIT_DBPATH`、`GOODHABIT_DAY_TZ`、`GOODHABIT_TRUST_PROXY`、`GOODHABIT_SECURE_COOKIE`、`GOODHABIT_READ_POOL`；env 为底、flag 覆盖）。默认：`127.0.0.1:8080`、`good-habit.db`、`ReadPool=5`（clamp 4–6）。

- [ ] **Step 1: 写失败测试**

```go
// internal/config/config_test.go
func TestLoadDefaults(t *testing.T)      // nil args → Addr=="127.0.0.1:8080"、DBPath=="good-habit.db"、ReadPool==5
func TestLoadEnvOverride(t *testing.T)   // t.Setenv GOODHABIT_DBPATH="x.db"、GOODHABIT_READ_POOL="6" → 生效
func TestReadPoolClamp(t *testing.T)     // 2→4、99→6
```

- [ ] **Step 2: 运行确认失败** — `go test ./internal/config/ -v`（包不存在/未定义）
- [ ] **Step 3: 实现 `config.go`** — `flag.NewFlagSet("good-habit", flag.ContinueOnError)`；`TrustProxy/SecureCookie` 接受 `1/true`
- [ ] **Step 4: 运行测试通过**
- [ ] **Step 5: `main.go` 最小版** — Load、`/healthz` 返回 200、`ListenAndServe`（Task 17 重写为全装配）
- [ ] **Step 6: `go mod init good-habit`，`go build ./... && go vet ./...` 全绿后提交**

```bash
git add go.mod cmd internal && git commit -m "chore: 项目骨架与配置加载"
```

### Task 2: 数据库层（ReadDB/WriteDB 拆分 + PRAGMA 基线）

**Files:**
- Create: `internal/db/db.go`、`internal/db/db_test.go`

**Interfaces:**
- Produces: `type DBs struct { Write, Read *sql.DB; WriteMu sync.Mutex }`；`db.Open(dbPath string, readPool int) (*DBs, error)`（**先开写池再开读池**）；`db.WriteDSN(dbPath string) string`、`db.ReadDSN(dbPath string) string`；`func (d *DBs) WithTx(fn func(tx *sql.Tx) error) error`（锁 WriteMu → Begin → fn → Commit/Rollback）；`func (d *DBs) Close() error`。
- Consumes: `go get modernc.org/sqlite`。

- [ ] **Step 1: 写失败测试**（每个用例 `db.Open(filepath.Join(t.TempDir(), "t.db"), 5)`）
  - `TestPragmasEffective`：写池查询 `PRAGMA journal_mode`→`wal`、`wal_autocheckpoint`→0、`synchronous`→1、`mmap_size`→134217728、`cache_size`→-32000；读池查询 `PRAGMA query_only`→1。
  - `TestReadPoolCannotWrite`：读池执行 `INSERT` 报错（错误串含 `readonly` 或 `read-only`）。
  - `TestWriteTxCommit`：WithTx 建表插行 → 读池能查到。
  - `TestReadPoolConcurrent`：6 个 goroutine 并发读全部成功。
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 实现 `db.go`** — DSN 按 Global Constraints 原文；写池 `SetMaxOpenConns(1)`+`SetMaxIdleConns(1)`，读池 `SetMaxOpenConns(readPool)`+`SetMaxIdleConns(readPool)`
- [ ] **Step 4: `go test ./internal/db/ -race -v` 通过后提交** `git add internal/db go.mod go.sum && git commit -m "feat(db): 读写池拆分与 PRAGMA 基线"`

### Task 3: 迁移框架与 0001 schema

**Files:**
- Create: `migrations/embed.go`（`package migrations; //go:embed *.sql; var FS embed.FS`）、`migrations/0001_init.sql`、`internal/db/migrate.go`、`internal/db/migrate_test.go`

**Interfaces:**
- Produces: `migrations.FS`；`func (d *DBs) Migrate(fsys fs.FS) error` —— 遍历根目录 `*.sql` 按文件名排序，版本号 = 文件名前缀数字（`0001_init.sql` → 1），已在 `schema_migrations` 记录的跳过，未应用的在 WithTx 内整文件执行并记账。
- Consumes: Task 2 `WithTx`。

- [ ] **Step 1: 写失败测试**
  - `TestMigrateFresh`：`Migrate(migrations.FS)` 后 `sqlite_master` 含 families/users/reward_items/prizes/draws/ledger/sessions/schema_migrations 八张表。
  - `TestMigrateIdempotent`：二次 Migrate 无错，`schema_migrations` 恰 1 行。
- [ ] **Step 2: 运行确认失败**
- [ ] **Step 3: 落 `0001_init.sql`** — 设计文档 §7 的 DDL **原样照抄**（含全部 CHECK / 外键 / 索引）
- [ ] **Step 4: 实现 `migrate.go`**
- [ ] **Step 5: 通过后提交** `git add migrations internal/db && git commit -m "feat(db): embed 迁移与 0001 schema"`

### Task 4: 落盘算子（ledger.Op）

**Files:**
- Create: `internal/ledger/ops.go`、`internal/ledger/ops_test.go`

**Interfaces:**
- Produces:

```go
type Op interface{ Apply(tx *sql.Tx) error }
type EarnOp struct { UUID string; FamilyID, ChildID, RewardItemID int64; Delta, BalanceAfter int64; Note string; CreatedAt int64 }
type AdjustOp struct { UUID string; FamilyID, ChildID int64; Delta, BalanceAfter int64; Note string; CreatedAt int64 }
type DrawOp struct {
    UUID string; FamilyID, ChildID int64
    Mode string /* "single"|"double" */; Cost int64
    Prize0ID, Prize1ID int64 /* Prize1ID=0 表示单抽 */
    State string /* "pending_choice"|"completed" */; Picked *int /* nil=未选 */
    Delta, BalanceAfter int64; CreatedAt int64
}
type SessionUpsertOp struct { TokenHash string; UserID int64; ExpiresAt, CreatedAt int64 }
type SessionDeleteOp struct { TokenHash string }
```

  Apply 语义（同一事务内）：EarnOp/AdjustOp = INSERT ledger 行 + `UPDATE users SET balance=? WHERE id=?`；DrawOp = 先 INSERT draws 行、取 `LastInsertId` 作 `ref_draw_id`，再 INSERT ledger 行，再 UPDATE users.balance；Session 两个 Op 只动 sessions 表。
- Consumes: Task 2 `WithTx`；Task 3 schema。

- [ ] **Step 1: 写失败测试**（测试内用 WithTx 直插 family/user/reward_item/prize 种子行）
  - `TestEarnOpApplies`：EarnOp{Delta:7, BalanceAfter:7} → ledger 行字段全对、users.balance==7。
  - `TestDrawOps`：single（Picked=&0, State=completed）→ draws+ledger 行齐、`ref_draw_id` 指向该 draws.id、余额扣减；double pending（Picked=nil, Prize1ID≠0）→ state='pending_choice'、picked IS NULL。
  - `TestSessionOps`：Upsert 后行在、Delete 后消失。
  - `TestLedgerChain`：连续两个 EarnOp（BalanceAfter=5、10）→ 按 id 排序的余额链自洽。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `ops.go`**
- [ ] **Step 4: `go test ./internal/ledger/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/ledger && git commit -m "feat(ledger): 组提交落盘算子"`

### Task 5: 异步组提交写入器

**Files:**
- Create: `internal/writer/writer.go`、`internal/writer/writer_test.go`

**Interfaces:**
- Produces: `writer.New(dbs *db.DBs, opts ...Option) *Writer`；Option：`WithFlushInterval(time.Duration)`（默认 15ms）、`WithMaxBatch(int)`（默认 64）；`func (w *Writer) Enqueue(op ledger.Op)`（chan 容量 8192、满则阻塞）；`func (w *Writer) Start()`；`func (w *Writer) Stop() error`（置 stopping → 循环排空 → 最后一批提交 → 返回）；`func (w *Writer) Commits() int64`（atomic 计数，测试钩子）。
- Consumes: Task 2 `DBs/WithTx`、Task 4 `Op`。

- [ ] **Step 1: 写失败测试**（每用例独立 temp DB，`defer writer.Stop()`）
  - `TestFlushWithinInterval`：1 个 EarnOp、`WithFlushInterval(30ms)`，等 150ms → ledger 行存在、`Commits() ≥ 1`。
  - `TestBatchUpTo64`：200 个 Op、interval 1h，入队完立即 `Stop()` → 全部落盘且 `Commits() ≤ 4`（200/64 向上取整）。
  - `TestDrainOnStop`：50 个 Op、interval 1h，入队完立即 `Stop()` → 50 行全在。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `writer.go`** — 主循环 select：`op := <-ch` 攒批（满 maxBatch 即 flush）；`<-ticker.C` 批非空即 flush；`<-stopCh` → 排空循环取尽后 flush 收尾。每批 flush = `dbs.WithTx(逐个 op.Apply)`。`Enqueue` 在 `Stop` 之后调用属编程错误（注释注明）。
- [ ] **Step 4: `go test ./internal/writer/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/writer && git commit -m "feat(writer): 8192 队列与 64 条/15ms 组提交"`

### Task 6: 内存热路径 Store

**Files:**
- Create: `internal/store/store.go`、`internal/store/load.go`、`internal/store/store_test.go`

**Interfaces:**
- Produces:

```go
type RewardItem struct{ ID int64; Title string; Points int64 }
type Prize struct{ ID int64; Name string; Emoji string }
type FamilyConfig struct{ FamilyID int64; Items []RewardItem; Prizes []Prize; SingleCost, DoubleCost int64 }
type Session struct{ TokenHash string; UserID, FamilyID int64; Role string; ExpiresAt int64 }
type ChildAcct struct { mu sync.Mutex; Balance int64; PendingUUID string; PendingDeadline int64; Unfulfilled int64 }

func New() *Store
func (s *Store) Child(id int64) *ChildAcct                       // 懒创建零值
func (s *Store) Family(familyID int64) *FamilyConfig             // 未加载返回 nil
func (s *Store) SetFamily(fc *FamilyConfig)
func (s *Store) PutSession(sess Session)
func (s *Store) GetSession(tokenHash string) (Session, bool)
func (s *Store) DeleteSession(tokenHash string)
func (s *Store) BumpEarnsToday(familyID int64, dayKey string)
func (s *Store) EarnsToday(familyID int64, dayKey string) int64
func (s *Store) LoadAll(dbs *db.DBs) error                       // 启动重建
func (s *Store) ReloadFamily(dbs *db.DBs, familyID int64) error  // 家长 CRUD 后单家庭重载
```

  LoadAll：遍历 active child 用户，比对 `users.balance` 与 `SUM(ledger.delta)`，不一致以 SUM 为准（WithTx 修复 + slog 警告）；加载未过期会话；逐家庭加载 active 积分项/奖品（按 sort）+ 定价 → SetFamily。
- Consumes: Task 2 `DBs/WithTx`。

- [ ] **Step 1: 写失败测试**（测试内直插 2 个家庭、孩子、账单、过期/未过期会话、积分项/奖品）
  - `TestLoadAllRebuild`：余额、会话、快照（active-only、按 sort、SingleCost/DoubleCost）全对；过期会话不出现。
  - `TestLoadAllRepairsBalance`：users.balance=100 而 SUM=90 → LoadAll 后 Child(id).Balance==90 且 DB 行==90。
  - `TestDayCounter`：BumpEarnsToday("2026-11-25")×3 → EarnsToday==3、其它 dayKey==0。
  - `TestSessionMap`：Put/Get/Delete 往返。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `store.go` + `load.go`**（配置快照用 `atomic.Pointer` 存 `map[familyID]*FamilyConfig` 整体替换）
- [ ] **Step 4: `go test ./internal/store/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/store && git commit -m "feat(store): 余额/会话/配置快照/今日计数内存化"`

### Task 7: 认证服务

**Files:**
- Create: `internal/auth/auth.go`、`internal/auth/auth_test.go`

**Interfaces:**
- Produces:

```go
func HashPassword(pw string) (string, error)   // bcrypt cost 10
func CheckPassword(hash, pw string) bool
func NewToken() (token, tokenHash string, err error)  // 32B crypto/rand → base64url；sha256(token) hex
var ErrDuplicateUsername, ErrBadCredentials, ErrLocked, ErrValidation = errors.New(...)

type Service struct { DBs *db.DBs; Store *store.Store; Enqueue func(ledger.Op); LockWindow time.Duration /* 测试可调，默认 10min */ }
func (a *Service) Register(username, password, familyName string) (store.Session, error)
func (a *Service) Login(username, password string) (store.Session, error)
func (a *Service) Logout(tokenHash string)
const SessionTTL = 30 * 24 * time.Hour
```

  Register：校验（用户名小写化后 `^[a-z0-9_]{3,32}$`、密码 ≥8、家庭名默认"我们家"）→ WithTx 插 families+users(role=parent) → PutSession + Enqueue(SessionUpsertOp)。Login：读池查 user（active=1）→ bcrypt 比对；同账号连续失败 5 次锁 LockWindow（内存 map + mutex）；成功清零计数并 PutSession + Enqueue。Logout：DeleteSession + Enqueue(SessionDeleteOp)。
- Consumes: Task 2/5/6；`go get golang.org/x/crypto`。

- [ ] **Step 1: 写失败测试**
  - `TestRegisterAndLogin`：注册 → DB 有 family+user、store 有会话；登录成功返回会话。
  - `TestDuplicateUsername`：二次注册 → ErrDuplicateUsername。
  - `TestLoginLockout`：错密码 5 次 → ErrLocked；LockWindow=50ms 的 Service 等 100ms 后可再尝试。
  - `TestTokenShape`：token 为 43 字符 base64url、tokenHash 为 64 位 hex。
  - `TestValidation`：坏用户名（大写/短/非法字符）、短密码 → ErrValidation。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `auth.go`**
- [ ] **Step 4: `go test ./internal/auth/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/auth go.mod go.sum && git commit -m "feat(auth): bcrypt/会话/注册登录与失败锁定"`

### Task 8: 内存限流器

**Files:**
- Create: `internal/ratelimit/ratelimit.go`、`internal/ratelimit/ratelimit_test.go`

**Interfaces:**
- Produces: `ratelimit.New(window time.Duration, max int64) *Limiter`；`func (l *Limiter) Allow(key string) bool`（定长窗口：`epoch = now.UnixMilli()/windowMs` 分桶，`map[string]*slot` + mutex，slot 内 atomic 计数）；`func (l *Limiter) Stop()`（停 30s 清扫协程）。

- [ ] **Step 1: 写失败测试**
  - `TestAllowLimit`（window 60s, max 3）：同 key 3 次 true、第 4 次 false；异 key 互不影响。
  - `TestWindowRollover`：window 50ms、max 1：首次 true，sleep 120ms 后再 true。
  - `TestConcurrentAllow`：100 goroutine 同 key、max 10 → 恰 10 个 true（`-race`）。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `ratelimit.go`**
- [ ] **Step 4: `go test ./internal/ratelimit/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/ratelimit && git commit -m "feat(ratelimit): 定长窗口内存限流"`

### Task 9: HTTP 骨架（中间件链 / 错误封装 / 角色门）

**Files:**
- Create: `internal/httpapi/server.go`、`internal/httpapi/middleware.go`、`internal/httpapi/errors.go`、`internal/httpapi/server_test.go`

**Interfaces:**
- Produces:

```go
type Deps struct { Cfg config.Config; DBs *db.DBs; Store *store.Store; Auth *auth.Service
    Enqueue func(ledger.Op); RateGeneral int64 /* 0 → 默认 240/分钟 */ }
type server struct{ deps Deps }   // 内部类型；后续所有 handler 方法挂它上面
func newServer(deps Deps) *server
func (s *server) routes() *http.ServeMux
func New(deps Deps) *http.ServeMux            // = newServer(deps).routes()
type APIError struct{ Status int; Code, Msg string }
func writeJSON(w http.ResponseWriter, status int, v any)
func writeErr(w http.ResponseWriter, status int, code, msg string)  // {"error":{"code":...,"message":...}}
func SessionFrom(r *http.Request) (store.Session, bool)
func RequireRole(role string, h http.HandlerFunc) http.HandlerFunc  // 401 unauthenticated / 403 forbidden
func setSessionCookie(w http.ResponseWriter, token string)
func clearSessionCookie(w http.ResponseWriter)
```

  中间件链（`routes()` 上包一层）：请求 ID（8B hex）→ slog 访问日志 → 全局限流（IP 维度；`Cfg.TrustProxy` 时取 `X-Forwarded-For` 首段，否则 RemoteAddr host）→ 会话加载（cookie `gh_session` → sha256 hex → store）。`GET /healthz` → 200。
- Consumes: Task 6/7/8、Task 1 config。

- [ ] **Step 1: 写失败测试**（直接 `mux.ServeHTTP(rr, req)`；受保护路由用测试内注册的 dummy handler）
  - `TestHealthz`：200。
  - `TestUnauthEnvelope`：未登录访问受保护路由 → 401，body 形如 `{"error":{"code":"unauthenticated","message":...}}`。
  - `TestRoleGate`：parent 会话访问 RequireRole("child") 路由 → 403；child 会话访问 RequireRole("parent") → 403。
  - `TestRateLimitGeneral`：`RateGeneral=3` → 第 4 次请求 429 `rate_limited`。
  - `TestCookieAttrs`：setSessionCookie 输出含 HttpOnly、SameSite=Strict、Path=/、MaxAge=2592000。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 server/middleware/errors**
- [ ] **Step 4: `go test ./internal/httpapi/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/httpapi && git commit -m "feat(httpapi): 中间件链/错误封装/角色门"`

### Task 10: 注册 / 登录 / 登出 / me 端点

**Files:**
- Create: `internal/httpapi/auth_handlers.go`、`internal/httpapi/auth_handlers_test.go`

**Interfaces:**
- Produces（路由，全部经 Task 9 链）：
  - `POST /api/auth/register` body `{username,password,family_name?}` → 200 `{user:{id,username,displayName,role,familyId}}` + Set-Cookie；409 `duplicate_username`；422 `invalid_username` / `short_password`；429 `rate_limited`（auth 专用限流：每 IP 10 次/分钟，独立 Limiter 实例）。
  - `POST /api/auth/login` → 同上；401 `bad_credentials`；409 `account_locked`。
  - `POST /api/auth/logout` → 204 + 清 cookie。
  - `GET /api/me` → 200 `{user:{...}}`；401。
- Consumes: Task 7 `auth.Service` 全部签名；Task 9 cookie/错误工具。

- [ ] **Step 1: 写失败测试**：注册→me→logout→me(401) 全链（cookie 从 recorder 取出回带）；重复用户名 409 码；错密码 401 `bad_credentials`；同账号连续 5 次后 409 `account_locked`；短密码 422。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `auth_handlers.go`**
- [ ] **Step 4: `go test ./internal/httpapi/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/httpapi && git commit -m "feat(httpapi): 注册/登录/登出/me"`

### Task 11: 孩子管理端点（家长）

**Files:**
- Create: `internal/httpapi/children_handlers.go`、`internal/httpapi/children_handlers_test.go`

**Interfaces:**
- Produces：
  - `GET /api/children` → `{children:[{id,username,displayName,balance,active}]}`（余额读内存）。
  - `POST /api/children` `{username,password,displayName}` → 200 同形状（WithTx 插 child user；`store.Child(id)` 就绪）。
  - `PATCH /api/children/{id}` `{displayName? password? active?}` → 200；改名 / 重置密码（bcrypt）/ 停用（软删除）。
  - 越权：操作非本家庭 child → 404 `not_found`；参数校验失败 → 422。
- Consumes: Task 7 bcrypt、Task 6 store、Task 2 WithTx。

- [ ] **Step 1: 写失败测试**
  - `TestChildLifecycle`：建孩子（GET 出现、余额 0）→ 改名 → 重置密码（旧密码登录 401、新密码 200）→ 停用（登录 409 `account_inactive`、GET 中 active=false）。
  - `TestChildFamilyIsolation`：家庭 B 的 parent PATCH 家庭 A 的孩子 → 404。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `children_handlers.go`**
- [ ] **Step 4: `go test ./internal/httpapi/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/httpapi && git commit -m "feat(httpapi): 孩子子账号管理"`

### Task 12: 积分项 / 奖品 / 定价端点 + 快照刷新

**Files:**
- Create: `internal/httpapi/items_handlers.go`、`internal/httpapi/prizes_handlers.go`、`internal/httpapi/wheelconfig_handlers.go` 及对应测试

**Interfaces:**
- Produces:
  - `GET/POST /api/reward-items`、`PATCH/DELETE /api/reward-items/{id}`（title、points>0、active、sort；DELETE=软删除）。
  - `GET/POST /api/prizes`、`PATCH/DELETE /api/prizes/{id}`（name、emoji 默认 🎁、active、sort）。
  - `GET/PUT /api/wheel-config` `{singleCost,doubleCost}`（PUT 校验 `1 ≤ single ≤ double ≤ 999`，违规 422）。
  - 每个 mutation 成功后调 `Store.ReloadFamily(dbs, familyID)` 原子替换内存快照。
- Consumes: Task 6 `ReloadFamily/Family`、Task 2 WithTx。

- [ ] **Step 1: 写失败测试**
  - `TestRewardItemCRUD`：建/改/停用/软删往返；GET 列出全部（含 active=false 的项）。
  - `TestPrizeCRUD`：同构 + emoji 缺省 🎁。
  - `TestWheelConfigValidation`：single=0、double<single、double=1000 → 422；合法值 → 200 且 `Store.Family(fid).SingleCost/DoubleCost` 更新。
  - `TestSnapshotRefresh`：新增一个 active item 后 `Store.Family(fid).Items` 含它（active-only、按 sort）。
  - `TestFamilyIsolation`：B 家读写 A 家资源 → 404。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现三个 handlers 文件**
- [ ] **Step 4: `go test ./internal/httpapi/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/httpapi && git commit -m "feat(httpapi): 积分项/奖品/转盘定价与快照刷新"`

### Task 13: 种子数据导入端点

**Files:**
- Create: `internal/seed/seed.go`、`internal/seed/seed_test.go`、`internal/httpapi/bootstrap_handlers.go`、`internal/httpapi/bootstrap_handlers_test.go`

**Interfaces:**
- Produces: `seed.Data() (items []store.RewardItem, prizes []store.Prize)`（设计文档**附录 A 原文**：11 个积分项含分值、3 个奖品）；`POST /api/bootstrap`（parent）body `{items?,prizes?,singleCost?,doubleCost?}` 缺省用附录 A → 200 `{importedItems,importedPrizes}`；家庭已存在任何 reward_item 或 prize → 409 `already_bootstrapped`。导入走 WithTx，成功后 ReloadFamily。
- Consumes: Task 6、Task 12 的校验规则。

- [ ] **Step 1: 写失败测试**：默认导入后 DB 有 11+3 行；`seed.Data()` 断言精确标题与分值（如 "托管课写完作业"=7、"背诵1篇英语短文"=5、奖品恰为 水果糖1颗/巧克力1颗/自选零食1袋）；自定义 body 生效；二次导入 → 409。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 seed + handler**
- [ ] **Step 4: `go test ./internal/... -race` 通过**
- [ ] **Step 5: 提交** `git add internal && git commit -m "feat(httpapi): 附录A种子一键导入"`

### Task 14: 登记得分与手动调整（热路径）

**Files:**
- Create: `internal/httpapi/earnings_handlers.go`、`internal/httpapi/earnings_handlers_test.go`

**Interfaces:**
- Produces:
  - `POST /api/earnings`（parent）`{childId, rewardItemId, note?}` → 200 `{entry:{delta, balanceAfter, kind:"earn", itemTitle}}`。
  - `POST /api/adjustments`（parent）`{childId, delta, note}`（note 必填，delta≠0、可负）→ 200 `{entry:{delta, balanceAfter, kind:"adjust"}}`。
  - 核心函数：`func (s *server) applyLedger(familyID, childID, delta int64, mkOp func(balanceAfter int64, uuid string) ledger.Op) (int64, *APIError)` —— 在 `child.mu` 内：校验 `balance+delta ≥ 0` → 变更 → `Enqueue(mkOp(...))` → 返回新余额；earn 路径另调 `BumpEarnsToday`。
  - 失败码：item/child 不存在或停用 → 404/422；结果余额为负 → 409 `insufficient_points`（零副作用）。
- Consumes: Task 4 `EarnOp/AdjustOp`、Task 5 `Enqueue`、Task 6 store。

- [ ] **Step 1: 写失败测试**
  - `TestEarnImmediateAndPersisted`：响应即时新余额；writer.Stop 后 ledger 行落盘、`balance_after` 与 `users.balance` 均匹配。
  - `TestAdjustRejectNegative`：对余额 50 调 -100 → 409、账单行数 0、内存余额不变。
  - `TestEarnInactiveItem`：停用 item → 422。
  - `TestConcurrentEarn`（-race）：50 并发各 +1 → 余额恰 +50、落盘 SUM 一致。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `earnings_handlers.go`**
- [ ] **Step 4: `go test ./internal/httpapi/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/httpapi && git commit -m "feat(httpapi): 登记得分与手动调整热路径"`

### Task 15: 抽奖（单抽 / 双抽 / 选择 / 发放 / 清扫）

**Files:**
- Create: `internal/httpapi/draw_handlers.go`、`internal/httpapi/draw_handlers_test.go`、`internal/sweep/sweep.go`、`internal/sweep/sweep_test.go`

**Interfaces:**
- Produces:

```go
type drawService struct{ /* deps: Store, Enqueue, DBs, Cfg, childLimiter */ }
func newDrawService(deps Deps) *drawService  // 内置 ratelimit.New(time.Minute, 30)，key=strconv.FormatInt(childID)
func (ds *drawService) Execute(familyID, childID int64, mode string) (DrawResp, *APIError)
```

  `POST /api/draws`（child）`{mode}` 响应形状：
  - single：`{draw:{uuid,mode:"single",state:"completed",prize:{id,name,emoji}},entry:{delta,balanceAfter,kind:"draw"}}`
  - double：`{draw:{uuid,mode:"double",state:"pending_choice",choices:[prize,prize]},entry:{delta,balanceAfter,kind:"draw"}}`
  - 失败码：`pending_choice_exists` / `insufficient_points` / `no_active_prizes`（409）、`bad_mode`（422）、`rate_limited`（429，每孩子 30 次/分钟，设计 §6.8）。
  Execute 流程（取锁前先过 childLimiter；其余全部在 `child.mu` 内）：`Family(familyID)` 快照 → `PendingUUID != ""` 拒 → active prizes 为空拒 → 按 mode 取 cost → 余额不足拒 → `randInt`（crypto/rand）独立选 1 或 2 个 prize → `Balance -= cost` → double 置 `PendingUUID/PendingDeadline(now+48h)`、single 直接 `Unfulfilled++` → `Enqueue(DrawOp{...})` → 构造响应。
  - `POST /api/draws/{uuid}/choose`（child）`{pick: 0|1}`：WithTx `UPDATE draws SET picked=?, state='completed' WHERE uuid=? AND family_id=? AND state='pending_choice'`（影响 0 行 → 404）→ 内存清 PendingUUID、`Unfulfilled++` → 200 `{draw:{uuid,prize:选中,state:"completed"}}`。
  - `PATCH /api/draws/{uuid}/fulfill`（parent）：WithTx 置 `fulfilled_at`（已发放 → 409 `already_fulfilled`）→ `Unfulfilled--` → 200。
  - `sweep.Run(dbs, st) (int, error)`：`UPDATE draws SET picked=0, state='completed' WHERE state='pending_choice' AND created_at < now-48h`，取回受影响 `(uuid, child_id)` 逐个清内存指针并 `Unfulfilled++`；返回条数。
- Consumes: Task 4 `DrawOp`、Task 6、Task 14 `applyLedger`（扣费复用或同构内联）。

- [ ] **Step 1: 写失败测试**
  - `TestSingleDrawFlow`：内存扣 5、响应含 prize；Stop 后 draws+ledger 行齐（ref_draw_id 对、users.balance 对）、Unfulfilled=1；fulfill → fulfilled_at 非空、Unfulfilled=0、重复 fulfill → 409。
  - `TestDoubleDrawFlow`：响应两个 choices + pending → 再抽 → 409 `pending_choice_exists` → choose 1 → completed、prize=choices[1] → fulfill OK。
  - `TestEmptyPrizePool`：全部奖品停用 → 409 `no_active_prizes`，账单 0 行、余额不变。
  - `TestInsufficientBalance`：余额 3、cost 5 → 409、零副作用。
  - `TestDrawRateLimit`：第 31 次同孩子 Execute（余额足够）→ 429 `rate_limited`、零副作用。
  - `TestConcurrentDraws`（-race）：余额=5×cost，10 并发 Execute → 恰 5 成功、终余额 0、ledger SUM = -5×cost×5。
  - `TestSweepPending`：直插一条 49h 前的 pending draw + 内存指针 → Run → picked=0、completed、指针清、Unfulfilled+1。
  - `TestChooseForeignDraw`：B 家孩子 choose A 家 uuid → 404。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 draw_handlers + sweep**
- [ ] **Step 4: `go test ./internal/httpapi/ ./internal/sweep/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal && git commit -m "feat(httpapi): 单抽/保险双抽/选择/发放与超时清扫"`

### Task 16: 读端点（guide / wheel / overview / 账单 / 待发放）

**Files:**
- Create: `internal/httpapi/read_handlers.go`、`internal/httpapi/read_handlers_test.go`

**Interfaces:**
- Produces:
  - `GET /api/guide`（child）→ `{items:[{id,title,points}]}`（内存快照）。
  - `GET /api/wheel`（child）→ `{prizes:[{id,name,emoji}], singleCost, doubleCost, balance, pending: null | {uuid, choices:[prize,prize], deadline}}`。
  - `GET /api/overview`（parent）→ `{children:[{id,displayName,balance,unfulfilled,hasPending}], todayEarns}`（内存：ChildAcct + EarnsToday；dayKey=`time.Now().In(loc).Format("2006-01-02")`，loc 按 `Cfg.DayTZ`，空则 time.Local）。
  - `GET /api/ledger`（child 自动限定自己；parent 可 `?childId=` 查全家）→ `{rows:[...], nextBefore}`；`?before=<id>&limit=30`（≤100）；行形状 `{id,createdAt,kind,delta,balanceAfter,itemTitle,prizeName,mode,note}`。
  - `GET /api/draws?state=unfulfilled&childId=`（parent）→ `{draws:[{uuid,childName,mode,cost,prizeName,state,fulfilledAt,createdAt}]}`。
  - Ledger 查询 SQL（ReadDB，join 已定）：

```sql
SELECT l.id, l.created_at, l.kind, l.delta, l.balance_after, l.note,
       ri.title, d.mode, d.picked, p.name
FROM ledger l
LEFT JOIN reward_items ri ON ri.id = l.ref_reward_item_id
LEFT JOIN draws d         ON d.id  = l.ref_draw_id
LEFT JOIN prizes p        ON p.id = CASE WHEN d.mode='single' THEN d.prize0_id
                                         WHEN d.picked=1    THEN d.prize1_id
                                         ELSE d.prize0_id END
WHERE l.family_id = ? AND (? = 0 OR l.child_id = ?) AND (? = 0 OR l.id < ?)
ORDER BY l.id DESC LIMIT ?
```
- Consumes: Task 6、Task 2 Read 池。

- [ ] **Step 1: 写失败测试**（直插多类 ledger/draws 行做夹具）
  - `TestLedgerPagination`：35 行、limit 30 → nextBefore=第 30 行 id；带 before 取余 5 行。
  - `TestLedgerDisplayJoin`：earn 行 itemTitle 正确；draw 行 prizeName=选中奖品、mode 正确；adjust 行 note 正确。
  - `TestLedgerIsolation`：B 家孩子/家长查 A 家 → rows 为空。
  - `TestWheelAndGuide`：payload 形状断言（含 pending 挂起场景）。
  - `TestOverview`：余额/Unfulfilled/todayEarns 与夹具一致。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `read_handlers.go`**
- [ ] **Step 4: `go test ./internal/httpapi/ -race` 通过**
- [ ] **Step 5: 提交** `git add internal/httpapi && git commit -m "feat(httpapi): 读路径与账单分页"`

### Task 17: 应用装配、优雅停机、reset-password CLI

**Files:**
- Create: `internal/app/app.go`、`internal/app/app_test.go`
- Modify: `cmd/good-habit/main.go`（重写为子命令分发）

**Interfaces:**
- Produces:

```go
type App struct{ /* dbs, store, writer, httpSrv, mux, sweepStop */ }
func New(cfg config.Config) (*App, error)   // Open → Migrate(migrations.FS) → sweep.Run → LoadAll → writer.Start → httpapi.New → http.Server
func (a *App) ListenAndServe() error        // 绑 cfg.Addr
func (a *App) Handler() http.Handler        // 测试用
func (a *App) Shutdown(ctx context.Context) error  // 1) httpSrv.Shutdown 2) writer.Stop 3) 停 sweep ticker 4) dbs.Close
func ResetPassword(dbs *db.DBs, username, newPw string) error  // CLI 与测试共用
```

  New 内起 1 小时 sweep ticker（sweep.Run 出错仅 slog）。main：`os.Args[1] == "reset-password"` 时走 `app.ResetPassword`（flags `-user`、`-new`，WithTx 更新 bcrypt hash），否则 `New(cfg)` + 信号监听（os.Interrupt / SIGTERM）→ `Shutdown(30s ctx)` → 退出。
- Consumes: 全部前序包；`migrations.FS`。

- [ ] **Step 1: 写失败测试**
  - `TestAppLifecycleAndDrain`：New（temp DB）→ 经 Handler() 完成注册+建孩子+earn → Shutdown → 直接查 DB：ledger 行在（Drain 生效）、sessions 行在。
  - `TestAppRestart`：Shutdown 后对同一路径二次 New → 用旧 cookie 访问 /api/me 200；孩子余额不变。
  - `TestResetPassword`：改密后旧密码登录 401、新密码 200。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `app.go` + 重写 `main.go`**
- [ ] **Step 4: `go test ./... -race` 全绿**
- [ ] **Step 5: 提交** `git add internal/app cmd && git commit -m "feat(app): 装配/优雅停机/reset-password CLI"`

### Task 18: 全旅程集成测试

**Files:**
- Create: `internal/app/journey_test.go`

**Interfaces:**
- Consumes: Task 17 `App`（New/Handler/Shutdown）。无新产出。

- [ ] **Step 1: 写测试 `TestFullJourney`**（一个测试串全链）：
  1. parent 注册 → bootstrap（附录 A）→ 建 2 个孩子；
  2. child1 登录 → earn×3（不同 item）→ 单抽 1 次 → 双抽 1 次 → choose → 家长 fulfill；
  3. 家长手动 adjust +20；child2 earn 1 次；
  4. 断言不变量：child1 的 ledger 按 id 排序满足 `balance_after[i] == balance_after[i-1] + delta[i]`；`SUM(delta) == users.balance == 内存余额`；每个 completed draw 恰有一条 -cost 的 ledger 行；所有余额 ≥ 0；
  5. overview 的 todayEarns、unfulfilled 正确；ledger 分页翻完无缺行；
  6. Shutdown → 二次 New → 余额/会话/待发放全部一致；双抽 pending 期间 Shutdown 重启 → `GET /api/wheel` 的 pending 仍在。
- [ ] **Step 2: `go test ./internal/app/ -race -run TestFullJourney -v` 通过**（此测试应直接通过；失败说明前序任务有洞，先修复）
- [ ] **Step 3: 提交** `git add internal/app && git commit -m "test(app): 全旅程/不变量/重启恢复"`

### Task 19: 前端脚手架（Vite + Vue3 + TS + 路由 + 客户端 + 登录注册）

**Files:**
- Create: `web/`（vite `vue-ts` 模板）、`web/vite.config.ts`、`web/src/{main.ts,router.ts,api.ts,session.ts}`、`web/src/views/{LoginView.vue,RegisterView.vue}` 及空壳视图、`web/src/styles/base.css`、`internal/webui/dist/.keep`（空占位）、`.gitignore` 追加规则

**Interfaces:**
- Produces:
  - `api.ts`：`export class ApiError extends Error { status: number; code: string }`；`export async function api<T>(method: string, path: string, body?: unknown): Promise<T>`（JSON、同源 cookie；非 2xx 抛 ApiError；401 → `location.href = '/login'`）。
  - `session.ts`：`export interface Me { id: number; username: string; displayName: string; role: 'parent' | 'child'; familyId: number }`；`useSession()` → `{ me: Ref<Me | null>; refresh(): Promise<void>; logout(): Promise<void> }`（模块级单例状态）。
  - `router.ts`：`/login`、`/register`、`/kid/{home,wheel,bills}`（meta.role=child）、`/parent/{overview,earn,children,items,prizes,draws,bills}`（meta.role=parent）、`/`（按 me.role 重定向）；守卫：无 me → /login、角色不符 → 自己首页。空壳视图文件本任务建好（Task 20–22 填充）。
  - `vite.config.ts`：`build.outDir: '../internal/webui/dist'`、`emptyOutDir: true`、`server.proxy: {'/api': 'http://127.0.0.1:8080'}`。
  - `.gitignore` 追加：`web/node_modules/`、`internal/webui/dist/*`、`!internal/webui/dist/.keep`（`.keep` 保证 Task 23 的 `//go:embed all:dist` 在未构建前端时也能编译）。
- Consumes: Task 10 的 API 形状（登录/注册 JSON 与错误码）。

- [ ] **Step 1: 脚手架**：仓库根 `npm create vite@latest web -- --template vue-ts` → `cd web && npm install && npm install vue-router@4` → 按上面契约落文件；登录/注册页中文表单（用户名/密码/家庭名），错误码映射中文提示（`duplicate_username`→"用户名已被占用"、`bad_credentials`→"用户名或密码不对" 等）。
- [ ] **Step 2: `cd web && npm run build` 通过**（vue-tsc + vite；产物覆盖到 `internal/webui/dist`）
- [ ] **Step 3: 冒烟（可选）**：`go run ./cmd/good-habit` + `cd web && npm run dev` → 浏览器完成注册/登录。
- [ ] **Step 4: 提交** `git add web .gitignore && git commit -m "feat(web): 脚手架/路由/API客户端/登录注册"`

### Task 20: 孩子端页面（首页 / 账单 / 待选横幅）

**Files:**
- Create: `web/src/views/kid/{HomeView.vue,BillsView.vue}`、`web/src/components/{BalanceCard.vue,GuideList.vue,PendingBanner.vue,BillRow.vue}`、`web/src/layouts/KidLayout.vue`（底部 Tab：首页/转盘/账单）

**Interfaces:**
- Consumes: `GET /api/guide`、`GET /api/wheel`（余额与 pending）、`GET /api/ledger`（分页），即 Task 16 的 JSON 形状。
- Produces: `BillRow.vue` props `{row: LedgerRow}`；`PendingBanner.vue` props `{pending: {uuid, choices, deadline}}`，点击跳 `/kid/wheel?resume=<uuid>`。移动优先（≤768px 底部 Tab、桌面顶部 Tab）。

- [ ] **Step 1: 实现页面与组件**（首页 = BalanceCard + GuideList + PendingBanner；账单 = BillRow 列表 + 滚动到底自动加载下一页）
- [ ] **Step 2: `cd web && npm run build` 通过**
- [ ] **Step 3: 手测清单**：孩子登录看到余额与得分指南 → 家长（另一浏览器）登记得分 → 孩子刷新后余额与账单新行（±分、当时余额列）正确。
- [ ] **Step 4: 提交** `git add web && git commit -m "feat(web): 孩子首页/账单/待选横幅"`

### Task 21: 转盘页（Canvas 动画 + 单抽 / 双抽 / 选择）

**Files:**
- Create: `web/src/views/kid/WheelView.vue`、`web/src/components/{WheelCanvas.vue,ResultModal.vue,ChoiceModal.vue}`

**Interfaces:**
- Produces:

```ts
// WheelCanvas.vue
props: { prizes: { id: number; name: string; emoji: string }[] }
defineExpose<{ spin(targetIndex: number): Promise<void> }>()
```

  Canvas 2D：扇区均分；固定调色板循环取色；emoji+名称沿半径排布；顶部固定指针；`spin(targetIndex)` 用 `requestAnimationFrame` 从当前角转到目标角（加 5 整圈，目标扇区中心对准顶部指针），4000ms `cubic-bezier(0.15, 0.85, 0.25, 1)` 缓出，动画结束 resolve。
  WheelView 流程：载入 `GET /api/wheel` → 渲染扇区与两档价格按钮（余额不足禁用）→ 单抽：`POST /api/draws {mode:'single'}` → 按 `prize.id` 在本地 prizes 找 index → `await spin(index)` → ResultModal（奖品+新余额）；双抽：POST → `spin(A)` → 展卡 A → 点"再转一次" → `spin(B)` → 展卡 B → ChoiceModal 二选一 → `POST /api/draws/{uuid}/choose {pick}` → ResultModal。URL 带 `?resume=` 时进页面直接用 `GET /api/wheel` 的 pending.choices 弹 ChoiceModal。
- Consumes: Task 15 的响应 JSON 形状（single/double/choose）。

- [ ] **Step 1: 实现 `WheelCanvas.vue`**（动画参数如上，内部代码自定但契约不变）
- [ ] **Step 2: 实现 WheelView + 两个 Modal**（错误码映射：`insufficient_points`→"积分不够啦"、`no_active_prizes`→"转盘还没有奖品"、`pending_choice_exists`→"先选完上一局"）
- [ ] **Step 3: `cd web && npm run build` 通过**
- [ ] **Step 4: 手测清单**：单抽动画落点=结果扇区；双抽两轮+选择；409 场景提示正确；手机竖屏转盘不超屏宽。
- [ ] **Step 5: 提交** `git add web && git commit -m "feat(web): 转盘抽奖与双抽选择"`

### Task 22: 家长端页面

**Files:**
- Create: `web/src/views/parent/{OverviewView.vue,EarnView.vue,ChildrenView.vue,ItemsView.vue,PrizesView.vue,DrawsView.vue,BillsView.vue}`、`web/src/layouts/ParentLayout.vue`（桌面侧栏、≤768px 折叠为顶部 Tab）

**Interfaces:**
- Consumes: Task 11/12/13/14/15/16 的全部 API 形状。
- Produces：`EarnView` = 选孩子（横滑卡）→ 积分项大按钮网格（title+分值）→ 可选备注 → 提交 → toast"已记 N 分"+余额刷新 + 手动调整入口（±delta 与必填原因）；`DrawsView` = 待发放列表（孩子名/奖品/时间）+「标记已发放」；`ItemsView`/`PrizesView` = CRUD 列表 + 新增/编辑弹窗；`ChildrenView` = 建孩子/改名/重置密码/停用；`OverviewView` = 各孩子余额卡 + todayEarns + 待发放数。

- [ ] **Step 1: 实现七个页面 + 布局**（账单页复用 Task 20 的 BillRow）
- [ ] **Step 2: `cd web && npm run build` 通过**
- [ ] **Step 3: 手测清单**：登记得分→孩子端余额即变；建孩子→孩子能登录；改定价→转盘页价格变；停用积分项→孩子指南消失；标记发放→待发放数减一。
- [ ] **Step 4: 提交** `git add web && git commit -m "feat(web): 家长端全套页面"`

### Task 23: 静态资源 embed 与构建脚本

**Files:**
- Create: `internal/webui/webui.go`（`package webui; //go:embed all:dist; var Dist embed.FS`）、`internal/httpapi/static.go`、`internal/httpapi/static_test.go`、`build.sh`、`build.ps1`
- Modify: `internal/app/app.go`（mux 上挂静态）、`.gitignore`

**Interfaces:**
- Produces: `httpapi.RegisterStatic(mux *http.ServeMux, dist fs.FS)` —— `/assets/` 走 `http.FileServerFS` + `Cache-Control: public, max-age=31536000, immutable`；其余非 `/api`、非 `/healthz` 的 GET 一律回 `dist/index.html`（SPA fallback，`Cache-Control: no-cache`）。
- Consumes: Task 19 的 `internal/webui/dist`。

- [ ] **Step 1: 写失败测试**（用 `fstest.MapFS` 造 dist：index.html + assets/a.js）
  - `TestStaticServesIndex`：GET / → 200 且为 index 内容。
  - `TestSPAFallback`：GET /kid/wheel → 200 且为 index 内容。
  - `TestAssetsImmutable`：GET /assets/a.js → 200 + immutable 缓存头。
  - `TestAPINotHijacked`：GET /api/nope → 401（非 index）。
- [ ] **Step 2: 确认失败**
- [ ] **Step 3: 实现 `webui.go` + `static.go` + app 接线（RegisterStatic 挂在 httpapi mux 之后）**
- [ ] **Step 4: `go test ./... -race` 全绿**
- [ ] **Step 5: 构建脚本**：`build.sh` / `build.ps1` = `cd web && npm ci && npm run build` → `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/good-habit-linux-amd64 ./cmd/good-habit`；执行 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` 验证交叉编译成功。
- [ ] **Step 6: 冒烟**：`go run ./cmd/good-habit` → `curl http://127.0.0.1:8080/` 返回 index.html、`curl /healthz` 200。
- [ ] **Step 7: 提交** `git add internal/webui internal/httpapi internal/app build.sh build.ps1 .gitignore && git commit -m "feat(webui): embed 静态服务/SPA fallback/交叉编译脚本"`

### Task 24: 部署件（systemd / Litestream / restore / README）

**Files:**
- Create: `deploy/good-habit.service`、`deploy/litestream.yml`、`deploy/litestream.service`、`deploy/restore.sh`、`deploy/README.md`

**Interfaces:**
- Consumes: 设计文档 §13 全部数值。无代码产出。

- [ ] **Step 1: 落四个部署文件**（数值照 §13）：
  - `good-habit.service`：`User=good-habit`；`ExecStart=/usr/local/bin/good-habit -addr 127.0.0.1:8080 -db /var/lib/good-habit/app.db`；`Restart=on-failure`；`KillSignal=SIGTERM`；`TimeoutStopSec=30`；`NoNewPrivileges=yes`、`ProtectSystem=strict`、`ReadWritePaths=/var/lib/good-habit`、`PrivateTmp=yes`。
  - `litestream.yml`：db 路径同上；`sync-interval: 10s`；`s3` 端点 `https://<ACCOUNT>.r2.cloudflarestorage.com`（占位）；bucket 占位；密钥走环境变量 `LITESTREAM_ACCESS_KEY_ID` / `LITESTREAM_SECRET_ACCESS_KEY`；`retention: 720h`。
  - `litestream.service`：`After=good-habit.service`，独立重启。
  - `restore.sh`：停应用 → 确认提示 → `litestream restore -o /var/lib/good-habit/app.db`（支持 `-timestamp` 参数透传）→ 起应用。
- [ ] **Step 2: `bash -n deploy/restore.sh` 语法通过**；README 覆盖：服务器初始化、二进制放置、R2 建桶与密钥、TLS 前置（隧道/反代）与 `GOODHABIT_TRUST_PROXY=1`、恢复演练步骤、设计文档 §9 的丢失窗口说明。
- [ ] **Step 3: 对照设计文档 §13 逐项核对后提交** `git add deploy && git commit -m "chore(deploy): systemd/Litestream/restore/README"`

---

## 执行说明

- 任务严格按序执行（1 → 24）；每个任务内部步骤打勾推进。
- 后端任务通用验收：`go test ./... -race` 全绿 + `CGO_ENABLED=0 go build ./...` 成功；前端任务：`cd web && npm run build` 通过。
- 所有接口签名以各任务 Interfaces 块为准；与设计文档冲突时以设计文档为准，并在计划文件中追加修正记录。
