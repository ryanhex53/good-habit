# 好习惯（good-habit）— 家庭积分与转盘抽奖系统 · 设计文档

- 日期：2026-11-25
- 状态：**待用户批准**
- 评审：已安排独立 builder check

## 0. 已由用户拍板的决策

| 决策点 | 结论 |
|---|---|
| 前端形态 | **Vue 3 + Vite**（TypeScript），构建产物 dist 由 Go `embed.FS` 打包 |
| 注册范围 | **多家庭开放注册**，各家庭数据完全隔离 |
| 转盘概率 | **均等概率**（每个奖品等机率，扇区均匀） |
| 后端架构 | 严格按用户的硬性要求执行（见 §6，逐条对应） |
| 界面语言 | 简体中文；桌面 + 手机响应式 |
| UI 框架 | **Tailwind CSS 4 + daisyUI 5**（纯 CSS 自托管、深浅双主题；2026-11-26 用户决定，取代「手写 CSS」裁量项） |

---

## 1. 意图与背景

家长希望用正向激励培养孩子的日常习惯：完成作业、早起、早睡、背诵等都折算成**积分**记到孩子名下；孩子用积分玩**转盘抽奖**换取小奖品（糖果、零食等）。系统由家长自己部署、自己管理，不依赖任何第三方服务即可运行；数据通过 Litestream 持续备份到 Cloudflare R2。

同时，本项目按**单机高并发 API 后端骨架**的标准来构筑（读写分离连接池、热路径全内存、异步组提交落盘），既是真实可用的家庭应用，也是一套可复用的 Go + SQLite 后端模板。

## 2. 目标 / 非目标 / 反目标

**目标**
1. 家长注册账号、为孩子开子账号，孩子有独立登录。
2. 家长维护积分项（示例：托管课写完作业 +7、被老师表扬 +5、9:45 上床睡觉 +5…），逐笔登记得分。
3. 家长维护转盘奖品池（水果糖 1 颗、巧克力 1 颗、自选零食 1 袋…）与抽奖定价。
4. 孩子玩转盘（有动画）：单抽扣积分随机中奖；**保险抽**多付积分抽两次、从两个结果中挑满意的一个。
5. 完整积分账单：每笔记录原因、±积分、所得奖品、**当时余额快照**。
6. 单个静态二进制部署（前端 + SQL 迁移全部 embed），systemd + Litestream(R2) 容灾。
7. 桌面与手机均可舒适使用。

**非目标**
- 多监护人共享管理一个家庭（一个家庭一个家长账号）。
- 奖品图片上传（奖品用 emoji + 名称表达）。
- 奖品库存/限量计数。
- 积分商城（直接花积分兑换固定商品）。
- 邮件验证 / 邮件找回密码（无 SMTP 依赖；密码重置走服务器本地 CLI）。
- 数据报表统计、多语言、移动 App、WebSocket 实时推送。
- 前端组件级自动化测试（只做类型检查 + 构建 + 少量核心流程手测）。

**反目标（做了就算失败）**
- 引入任何外部中间件：Redis、PostgreSQL、消息队列、ORM、Docker 运行时依赖。
- 依赖 CGO（必须能在 Windows 开发机上一条命令交叉编译出 Linux 静态二进制）。
- 积分可透支（余额永不小于 0）。
- 跨家庭数据可见（严格 family_id 隔离）。

## 3. 角色与账户

- **parent（家长）**：注册即创建一个"家庭"（family）。家长是该家庭唯一管理员。
- **child（孩子）**：由家长创建的子账号，有独立用户名 + 密码，属于且仅属于一个家庭。孩子只能看到自己家庭的数据、只能操作自己的积分。
- 用户名全局唯一（3–32 位，`a-z0-9_`，存储统一小写）；密码 ≥ 8 位。
- 家长忘记密码：在服务器上执行 CLI 子命令 `good-habit reset-password` 重置；孩子忘记密码：家长在后台重置。

## 4. 产品行为

### 4.1 注册与登录
- `POST /api/auth/register`：用户名 + 密码 + 家庭名（默认"我们家"）→ 创建 family + parent 用户 → 建立会话。
- `POST /api/auth/login`：统一登录入口（家长与孩子同一表单）→ 建立会话（HttpOnly Cookie，30 天滑动续期）。
- 登录失败按"IP + 账号"限速与锁定（见 §6.8）。
- 会话存内存（查询零磁盘）+ 异步落库（进程重启不掉登录态）。

### 4.2 孩子管理（家长）
- 创建/停用孩子账号，改名，重置密码。停用 = 软删除（`active=0`）：不能登录、不再出现在选择列表，但历史账单完整保留。

### 4.3 积分项（家长）
- CRUD 积分项：标题 + 分值（正整数）+ 排序 + 启用/停用。示例数据内置在首次注册的引导里可一键导入（见 §11 引导页）。
- 积分项读取走内存快照，不查库。

> 2026-11-27 增量：积分项可指定适用孩子，见 2026-11-27-point-item-child-scope-design.md。

### 4.4 登记得分（家长）
- 家长选孩子 + 积分项 → 一次登记 = 一条账单（`kind=earn`，delta = 分值）。可加备注。
- 高频操作：走热路径（内存余额原子加 + 异步落盘，见 §6.4/6.5）。

### 4.5 手动调整（家长）
- 家长对孩子手动 ±积分，必须填原因；写入 `kind=adjust` 账单。用于纠错、扣罚、奖励零头等。结果余额不可为负，否则整笔拒绝。

### 4.6 转盘配置（家长）
- 奖品池 CRUD：名称 + emoji（默认 🎁）+ 启用/停用 + 排序。
- 家庭级定价：`single_draw_cost`（默认 5）、`double_draw_cost`（默认 7），约束 `1 ≤ single ≤ double ≤ 999`。改价立即生效（内存快照原子替换）。
- 奖品池为空（无启用奖品）时抽奖接口返回明确错误，前端转盘显示"还没有奖品"。

### 4.7 抽奖（孩子，转盘动画）
- **单抽**：扣 `single_draw_cost` → 等概率随机抽一个启用奖品 → 必中。账单记 `kind=draw`，delta = -cost，关联 draw 记录。
- **保险抽（双抽）**：扣 `double_draw_cost` → 服务端一次算出两个（独立等概率）结果 A/B → 返回 `state=pending_choice`；前端转盘转两轮分别揭示 A、B，孩子挑一个（另一作废）；若中途离开，首页顶部常驻"有一局待选奖品"横幅可回来继续；**超 48 小时未选自动保留 A**（启动时与每小时清扫）。
- 扣费在发起时一次完成，选择环节不再扣/退分。
- **同一孩子同时至多一局未选**：存在 `pending_choice` 时发起新抽奖一律 409（`pending_choice_exists`），先完成选择（或超时自动保留 A）才能再抽。
- 余额不足：发起即拒绝（409），不产生任何账单。
- 每抽必中；家长想设"谢谢参与"，自己建一个 0 价值占位奖品即可。

### 4.8 奖品发放（家长）
- 中奖后 draw 处于"待发放"；家长在后台逐一标记"已发放"（`fulfilled_at`）。孩子端可见状态：待发放 / 已发放。

### 4.9 积分账单（家长、孩子各看各的）
- 按时间倒序分页，每行显示：时间、类型（得分项名称 / 抽奖 / 手动调整）、±积分、所得奖品（仅抽奖行）、**当时总积分**。
- 数据来自 ReadDB（见 §6.2 的时滞说明）；抽奖接口的响应本身携带该笔账单行，前端即时展示，翻页刷新最多滞后一个提交窗口（≤15ms 量级）。

### 4.10 余额规则
- 余额为整数，永不为负。所有加分/扣分操作在同一把孩子锁下"检查 + 变更 + 记账"原子完成（§6.4）。

## 5. 界面清单（桌面 + 手机响应式）

**公开页**
- `/login`、`/register`。

**孩子端 `/kid`**（手机优先，活泼风格，底部 Tab 导航）
- 首页：大号余额卡 + 得分指南（只读列出启用中的积分项：标题 + 分值，让孩子知道做什么能得分；点击无操作，登记永远由家长完成）+ "待选奖品"横幅（如有 pending_choice，至多一局）。
- 转盘页：Canvas 转盘（均分扇区，emoji + 名称，指针在顶），单抽/保险抽两个按钮（价格实时显示），~4s 缓动动画 → 结果弹窗；双抽为两轮动画 + 双卡片二选一。
- 账单页：时间线列表（±分、奖品、当时余额），下拉加载更多。

**家长端 `/parent`**（桌面侧栏 / 手机顶部 Tab，清爽工具风）
- 总览：各孩子余额卡、待发放奖品数、今日得分笔数。
- 孩子管理、积分项管理、奖品与定价、登记得分（高频，做成大按钮流程）、待发放列表、全家庭账单（按孩子筛选）。

**引导**：注册后首次进入提供"一键导入示例积分项/奖品"（就是需求里给的那两组）。

## 6. 架构（逐条对应硬性要求）

### 6.1 总体形态
- 原生 Go（≥1.23）+ 标准库 `net/http`（Go 1.22+ 方法路由），零 Web 框架、零 ORM、零外部中间件。
- 前端 Vue 3 + Vite 构建产物与 SQL 迁移脚本经 `embed.FS` 打进**单个二进制**；`go run` 即完整服务（开发期 Vite dev server 反代 `/api`）。
- SQLite 驱动选 **modernc.org/sqlite（纯 Go，无 CGO）**：Windows 开发机直接 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` 得到 Linux 静态二进制。
  - DSN 细节（`_query_only` 的拼写形式等）以钉住的 modernc 版本实测为准，语义不变：读池物理只读 + `query_only`，写池 `BEGIN IMMEDIATE`。实现阶段先写一个 DSN 冒烟测试验证。
- 目录结构：`cmd/good-habit`、`internal/{config,db,store,writer,auth,httpapi,ledger}`、`web/`（Vue 源码）、`migrations/`、`deploy/`。

### 6.2 数据库层（ReadDB / WriteDB 显式拆分）
启动顺序：先开 WriteDB（建 WAL/-shm、跑迁移），后开 ReadDB（避免只读连接遇到需恢复的 WAL）。

**WriteDB（唯一可写连接）**
- DSN：`file:<db>?_txlock=immediate&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=mmap_size(134217728)&_pragma=cache_size(-32000)&_pragma=wal_autocheckpoint(0)`
- `db.SetMaxOpenConns(1)`；Go 层再包一把 `sync.Mutex`（writeMu）串行化**所有**写事务（组提交批次与同步 CRUD 共用），SQLite 层 BUSY 不可能发生；busy_timeout 仅作为对 Litestream checkpoint 短暂持锁的兜底。

**ReadDB（并发读池）**
- DSN：`file:<db>?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)&_pragma=mmap_size(134217728)&_pragma=cache_size(-32000)`
- `SetMaxOpenConns(5)`（区间 4–6 取中，可用环境变量调），`SetMaxIdleConns(5)`。
- 语义要求（实现时验证拼写）：连接物理只读（`mode=ro`），并加 `query_only` 双保险。
- 账单、历史记录等非热路径读全部走 ReadDB。

### 6.3 迁移
- `migrations/0001_init.sql…` 嵌入二进制，启动时在 writeMu 下由 WriteDB 顺序执行，记录到 `schema_migrations`。幂等、可重启。

### 6.4 热路径内存化（`internal/store`）
全部高频读写驻留 Go 内存，磁盘零穿透：

| 状态 | 结构 | 并发控制 |
|---|---|---|
| 孩子余额 | `map[childID]*childAcct{mu sync.Mutex, balance int64, seq}` | **按孩子细粒度互斥锁**：检查+变更+生成账单快照+入队在锁内原子完成 |
| 配置快照 | `atomic.Pointer[FamilyConfig]`（积分项/奖品/定价整体副本） | 读无锁；家长 CRUD 走同步写库后整体替换指针（copy-on-write） |
| 会话 | `sync.Map[tokenHash]session` + TTL | 登录/登出异步落库，重启时从 DB 恢复 |
| 限流计数 | 分片 map + `atomic` 定长窗口计数 | 后台协程周期 GC |
| 待选抽奖（pending_choice） | childAcct 上单一指针（规则保证至多一局）+ DB | 供首页横幅与恢复 |

- 启动时从 DB 重建：余额（`users.balance`，并与账单 SUM 校验，不一致以账单为准并修复）、会话、全部家庭配置快照。

### 6.5 异步组提交落盘（`internal/writer`）
- 写入事件（登记得分、抽奖扣费+结果、手动调整、会话增删）投递到 `chan Event`，**容量 8192**。
- 单一后台写协程：凑满 **64 条**或距本批首条 **15ms**（ticker）即开一个事务（连接为 `_txlock=immediate`，`Begin` 即取写锁），批量应用后提交；批次拿 writeMu 与同步 CRUD 互斥。
- 事件内携带 enqueue 时在锁内算好的 `balance_after` 快照；同一孩子的事件因锁内入队而保持 FIFO，账单余额链单调自洽。
- 通道满时入队阻塞（退化为背压限速），API 延迟上升但不出错。
- 抽奖事件是复合事件：一个事务里同时落 `draws` 行 + `ledger` 行 + `users.balance` 更新，三者原子。

### 6.6 优雅停机
SIGTERM（systemd 停止）：
1. `http.Server.Shutdown`（先停止接新请求、等在途 handler 结束，handler 期间可继续入队）；
2. 通知 writer 停止取新批，**Drain**：循环排空通道内剩余事件并按批提交，直到通道空且无在途回调；
3. 关闭 ReadDB/WriteDB；退出。
**不通过 close(channel) 停止**（避免生产者写已关闭通道），用 stop 标志 + 排空检测。Drop 事件仅发生在 kill -9 / 断电（见 §9 丢失窗口）。

### 6.7 启动恢复
WriteDB 打开 → 迁移 → 清扫超时 pending_choice（>48h 自动选 A）→ 清理过期会话 → 重建内存态 → 开 ReadDB → 起监听。

### 6.8 限流（内存）
- 全局：每 IP 240 req/min；`/api/auth/*`：每 IP 10 req/min，且同账号连续失败 5 次锁 10 分钟。
- 抽奖：每孩子 30 次/min（并发双击由孩子锁的原子性天然兜底）。
- IP 来源默认 `RemoteAddr`；`GOODHABIT_TRUST_PROXY=1` 时信任 `X-Forwarded-For`（部署在隧道/反代后时开启）。

### 6.9 HTTP 层
- 标准库 `http.ServeMux`（`"POST /api/draws"` 等方法路由），JSON 编解码 `encoding/json`。
- 中间件链：请求 ID → 日志（结构化，stderr）→ 限流 → 会话鉴权 → 角色检查 → handler。
- 静态资源：`embed.FS` 提供 `index.html` + hash 命名资产（`Cache-Control: immutable`），SPA fallback 到 `index.html`。

## 7. 数据模型（迁移 0001，DDL 摘要）

```sql
CREATE TABLE families (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  single_draw_cost INTEGER NOT NULL DEFAULT 5,
  double_draw_cost INTEGER NOT NULL DEFAULT 7,
  created_at INTEGER NOT NULL
);

CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  family_id INTEGER NOT NULL REFERENCES families(id),
  role TEXT NOT NULL CHECK (role IN ('parent','child')),
  username TEXT NOT NULL UNIQUE,          -- 小写
  password_hash TEXT NOT NULL,            -- bcrypt
  display_name TEXT NOT NULL,
  balance INTEGER NOT NULL DEFAULT 0,     -- 仅 child；账单为准的冗余列，写协程同事务维护
  active INTEGER NOT NULL DEFAULT 1,      -- 软删除
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_users_family ON users(family_id, role, active);

CREATE TABLE reward_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  family_id INTEGER NOT NULL REFERENCES families(id),
  title TEXT NOT NULL,
  points INTEGER NOT NULL CHECK (points > 0),
  active INTEGER NOT NULL DEFAULT 1,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_reward_items_family ON reward_items(family_id, active, sort);

CREATE TABLE reward_item_children (
  reward_item_id INTEGER NOT NULL REFERENCES reward_items(id),
  child_id       INTEGER NOT NULL REFERENCES users(id),
  PRIMARY KEY (reward_item_id, child_id)
);
CREATE INDEX idx_reward_item_children_child ON reward_item_children(child_id);

CREATE TABLE prizes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  family_id INTEGER NOT NULL REFERENCES families(id),
  name TEXT NOT NULL,
  emoji TEXT NOT NULL DEFAULT '🎁',
  active INTEGER NOT NULL DEFAULT 1,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_prizes_family ON prizes(family_id, active, sort);

CREATE TABLE draws (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uuid TEXT NOT NULL UNIQUE,
  family_id INTEGER NOT NULL REFERENCES families(id),
  child_id INTEGER NOT NULL REFERENCES users(id),
  mode TEXT NOT NULL CHECK (mode IN ('single','double')),
  cost INTEGER NOT NULL,
  prize0_id INTEGER NOT NULL REFERENCES prizes(id),
  prize1_id INTEGER REFERENCES prizes(id),   -- 双抽第二个结果
  picked INTEGER,                            -- 0/1；单抽恒 0
  state TEXT NOT NULL CHECK (state IN ('pending_choice','completed')),
  fulfilled_at INTEGER,                      -- 家长标记发放时间
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_draws_family ON draws(family_id, state, fulfilled_at);
CREATE INDEX idx_draws_child ON draws(child_id, id);

CREATE TABLE ledger (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  uuid TEXT NOT NULL UNIQUE,
  family_id INTEGER NOT NULL REFERENCES families(id),
  child_id INTEGER NOT NULL REFERENCES users(id),
  delta INTEGER NOT NULL,                    -- ±
  balance_after INTEGER NOT NULL,            -- 当时余额快照
  kind TEXT NOT NULL CHECK (kind IN ('earn','draw','adjust')),
  ref_reward_item_id INTEGER REFERENCES reward_items(id),
  ref_draw_id INTEGER REFERENCES draws(id),
  note TEXT,                                 -- 手动调整原因/备注
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_ledger_child ON ledger(child_id, id);
CREATE INDEX idx_ledger_family ON ledger(family_id, id);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,               -- sha256(token)
  user_id INTEGER NOT NULL REFERENCES users(id),
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL
);
```

- 时间统一 Unix 毫秒；前端按浏览器本地时区展示。

## 8. API 一览（全部 JSON，`/api` 前缀）

**鉴权**
- `POST /auth/register` `{username,password,family_name?}`
- `POST /auth/login` `{username,password}` → Set-Cookie `gh_session`
- `POST /auth/logout`
- `GET /me` → 当前用户 + 家庭信息

**家长**
- `GET|POST /children`，`PATCH /children/:id`（改名/重置密码/停用）
- `GET|POST /reward-items`，`PATCH|DELETE /reward-items/:id`
- `POST /earnings` `{child_id, reward_item_id, note?}`（热路径）
- `POST /adjustments` `{child_id, delta, note}`（热路径）
- `GET|POST /prizes`，`PATCH|DELETE /prizes/:id`
- `GET|PUT /wheel-config` `{single_draw_cost,double_draw_cost}`
- `GET /draws?state=unfulfilled&child_id=`，`PATCH /draws/:id/fulfill`
- `GET /ledger?child_id=&before=&limit=30`（ReadDB，游标分页）
- `GET /overview`（各孩子余额、待发放数、今日得分笔数——内存直读；"今日"按服务器本地时区日界计算，可用 `GOODHABIT_DAY_TZ`（IANA 时区名）覆盖）

**孩子（仅 child 会话可用；家长不代替孩子抽奖，查看孩子数据走家长端接口）**
- `GET /guide`（只读：本家庭启用积分项列表，内存快照，供首页"得分指南"）
- `GET /wheel`（奖品快照 + 两档价格 + 余额 + pending_choice 抽奖如有）
- `POST /draws` `{mode:'single'|'double'}`（热路径；返回结果/两个候选）
- `POST /draws/:id/choose` `{pick:0|1}`
- `GET /ledger`（自己的）

**其它**
- `GET /healthz`（存活/就绪，供 systemd watchdog）
- 错误约定：`{"error":{"code":"...","message":"..."}}`，业务冲突（余额不足 `insufficient_points`、无启用奖品 `no_active_prizes`、有待选局 `pending_choice_exists`、用户名已存在、账号锁定）用 409/4xx，代码可枚举。
- 所有 `DELETE` 均为软删除（置 `active=0`，`PATCH` 可恢复）；账单与抽奖历史永不物理删除。

## 9. 一致性与容灾

**不变量（测试覆盖）**
1. 任意孩子：按 id 排序的 ledger 序列满足 `balance_after[i] = balance_after[i-1] + delta[i]`（首行 = delta）。
2. `users.balance` 恒等于该孩子 ledger 的 SUM（写协程同一事务维护 + 启动校验修复）。
3. 余额 ≥ 0 恒成立。
4. 每笔 `completed` 的 draw 恰有一条 `-cost` 的 ledger 行（同事务产生）。

**丢失窗口（如实声明）**
- 组提交：入队→落盘 ≤ 约 15ms / ≤64 条。**kill -9/断电**时该窗口内已返回给用户的操作（如刚中的奖品、刚登记的得分）可能丢失——内存态随进程消失，DB 仍自洽，重启后以 DB 为准。SIGTERM 优雅停机 **Drain 零丢失**。
- Litestream 每 10s 增量同步：R2 副本落后主库 ≤ 10s + 一个组提交窗口。整机数据丢失的灾难恢复最多损失约 10 秒 + 15ms 的写入。
- 该取舍是本架构（热路径不落盘）的固有属性，对本应用的积分/抽奖场景可接受；文档与部署说明中明示。

**WAL 与 Litestream**
- 应用侧 `PRAGMA wal_autocheckpoint = 0`（初始化时显式设置，checkpoint 完全交 Litestream），防止应用 checkpoint 截断 WAL 导致增量同步链断裂；WAL 增长由 Litestream 复制后的 checkpoint 管理。
- 备份链：app 单事务批次提交 → WAL → Litestream 10s 增量 → R2。

## 10. 认证与安全
- 密码 bcrypt（`golang.org/x/crypto/bcrypt`，cost 10；纯 Go 库，非中间件）。
- 会话 token 32 字节 crypto/rand → base64url；Cookie `HttpOnly + SameSite=Strict + Path=/`，`Secure` 由 `GOODHABIT_SECURE_COOKIE` 控制（TLS 反代后开启）；DB 只存 sha256(token)。30 天滑动续期。
- 权限：child 一律只能读写自己；parent 管理与查看本家庭全部数据；`/wheel`、`POST /draws`、`POST /draws/:id/choose` 为 child 会话专用（家长不代抽）。所有查询带 family_id 过滤（数据隔离的反目标红线）。
- 输入校验：用户名字符集、长度、密码长度、分值/价格范围、分页上限。
- 应用只说 HTTP；**公开部署必须在前面挂 TLS**（Cloudflare Tunnel/Caddy 皆可，属部署件而非应用依赖）；systemd 示例默认绑定 127.0.0.1。
- 注册无邮箱验证（自部署 + 限流下接受；反目标：引入 SMTP）。

## 11. 前端设计（Vue 3 + Vite + TypeScript）
- 依赖 `vue`、`vue-router` + 构建期 Tailwind CSS 4 / daisyUI 5（纯 CSS 组件类，无 JS 运行时，产物自托管）；状态用轻量 composable，不引 Pinia。
- 路由：`/login`、`/register`、`/kid/{home,wheel,bills}`、`/parent/{overview,earn,children,items,prizes,draws,bills}`；`/` 按角色跳转。
- 转盘：Canvas 2D 组件，扇区 = 启用奖品均分（含 emoji + 名称），`requestAnimationFrame` + 三次缓出 ~4s，结果扇区对准顶部指针；单抽一轮动画，保险抽两轮 + 双卡选择界面 + 确认弹窗。
- 手机优先：孩子端底部 Tab；家长端 lg（1024px）以下折叠为顶部 Tab。桌面居中内容栏。
- 构建：`web/` 下 `npm run build` → `web/dist` → Go `//go:embed all:web/dist`（Makefile/build 脚本固定"先前端后 Go"顺序；`web/dist` 不入库）。
- 开发：Vite dev server 代理 `/api` → `localhost:8080`；Go 侧 `go run` 即后端。

## 12. 构建与交叉编译
- `build.sh` / `build.ps1`：`npm ci && npm run build`（web/）→ `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o dist/good-habit-linux-amd64 ./cmd/good-habit`。Windows 本机开发：`go run ./cmd/good-habit`（modernc 无 CGO，NTFS 上 WAL 正常工作）。
- CLI 子命令：`good-habit reset-password -user <name>`（服务器本地执行，打印/设定新密码）。

## 13. 部署
**systemd（deploy/good-habit.service）**
- 专用用户 `good-habit`；`ExecStart=/usr/local/bin/good-habit -addr 127.0.0.1:8080 -db /var/lib/good-habit/app.db`；
- `Restart=on-failure`；`KillSignal=SIGTERM; TimeoutStopSec=30`（给 Drain 留时间）；
- 加固：`NoNewPrivileges`、`ProtectSystem=strict`、`ReadWritePaths=/var/lib/good-habit`、`PrivateTmp`。

**Litestream（deploy/litestream.yml + 独立 litestream.service）**
- 复制 `/var/lib/good-habit/app.db`；`sync-interval: 10s`；`s3` 副本指向 R2 兼容端点（`https://<account>.r2.cloudflarestorage.com`），bucket 与密钥走环境变量，保留如 30 天。
- litestream.service 与应用并行运行（`After=good-habit.service`、独立重启）。

**恢复（deploy/restore.sh）**
- 停应用 → `litestream restore -o /var/lib/good-habit/app.db`（带时间点参数可选）→ 校验 → 起应用；有交互确认，防覆盖现有库。

**部署文档（deploy/README.md）**：服务器初始化、R2 建桶与密钥、隧道/反代 TLS、trusted-proxy 说明、恢复演练步骤。

## 14. 测试策略
- **Go 单测**：余额链/下限/并发（`-race`）、组提交批触发（64 条/15ms，用可控时钟）、Drain 完整性、限流窗口、bcrypt/会话、迁移幂等。
- **集成测试**：`httptest` + 临时目录真库跑全旅程：注册→建孩子→导入示例→登记→单抽/双抽→选择→发放→账单校验（含不变量断言、重启恢复、SIGTERM Drain）。
- **基准**：`go test -bench` 覆盖热路径（draw/earn 入队 + 内存读），目标：不落盘路径 p99 < 1ms 量级；ReadDB 账单分页可并发。
- **前端**：`vue-tsc` 类型检查 + 构建通过；核心流程手测清单（移动/桌面各一遍）。

## 15. 决策归属清单

**用户已定**：见 §0。

**建造者裁量（本文档已选默认，实现时可直接采用）**
1. SQLite 驱动 modernc.org/sqlite（无 CGO）；DSN 里 `_query_only` 以 `_pragma=query_only(1)` 等价表达，语义以"读池物理只读+query_only、写池 immediate"为准。
2. ReadDB 连接数默认 5（4–6 可调）。
3. 双抽 pending_choice 模型：48h 超时自动选 A；启动+每小时清扫。
4. 前端 TypeScript；不引 Pinia；UI 用 daisyUI 5（原「手写 CSS」已于 2026-11-26 升级为 Tailwind/daisyUI，见 §0）。
5. 孩子软删除（历史保留）；积分项/奖品软删除同理。
6. 手动调整、CLI 重置密码、示例数据一键导入。
7. 限流阈值（§6.8）、会话 30 天、bcrypt cost 10、busy_timeout 数值。
8. `users.balance` 冗余列 + 启动对账（账单 SUM 为准）。
9. 待选双抽**同时至多一局**（存在 pending_choice 时拒绝新抽奖，409 `pending_choice_exists`）。
10. 抽奖/选择/转盘接口仅 child 会话可用；家长不代抽（家长查看孩子数据走家长端接口）。
11. "今日"= 服务器本地时区日界（`GOODHABIT_DAY_TZ` 可覆盖）；孩子首页的"得分指南"为只读列表，无打卡状态。
12. 引导导入的种子数据见附录 A（内容取自需求原文，标题照抄）。

**遗留待用户确认（builder check 后汇总提问）**
- 无。评审提出的 4 个缺口已全部闭合：种子数据取自需求原文（附录 A）；其余 3 项按上面第 9–11 条裁量定案（审阅时可否决）。

## 附录 A：一键导入种子数据（原文来自需求）

**积分项（11 项）**

| 标题 | 分值 |
|---|---|
| 托管课写完作业 | 7 |
| 上喜课经过审核 | 5 |
| 额外作业 | 10 |
| 被老师表扬 | 5 |
| 考试单科第一 | 10 |
| 1个鸡蛋 | 5 |
| 1盒牛奶 | 5 |
| 720完成起床 | 5 |
| 早读10分钟 | 5 |
| 945上床睡觉 | 5 |
| 背诵1篇英语短文 | 5 |

**转盘奖品（3 项）**

| 名称 | emoji |
|---|---|
| 水果糖1颗 | 🍬 |
| 巧克力1颗 | 🍫 |
| 自选零食1袋 | 🥨 |

**定价默认**：单抽 5 分、保险双抽 7 分（家长可改）。
