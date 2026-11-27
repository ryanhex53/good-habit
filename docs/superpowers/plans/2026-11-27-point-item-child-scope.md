# 积分项适用孩子范围 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让家长能给每个积分项指定适用孩子（默认所有孩子，可勾任意几个），登记页与孩子端得分指南只显示适用的项，后端拒绝不适用的登记。

**Architecture:** 新增 `reward_item_children` 关联表（零行 = 所有孩子），内存快照 `store.RewardItem` 带上 `ChildIDs` 并提供 `AppliesTo`；`handleEarning` / `handleGuide` 仍在内存里判定，热路径不加磁盘查询。积分项 CRUD 的请求体与响应多一个 `childIds`。前端抽一个 `ChildScopeChips` 组件（"所有孩子" + 每个孩子一颗 chip），放进积分项管理页的添加栏与新的 `ItemScopeDialog`；登记页按选中孩子过滤。

**Tech Stack:** Go 1.27（stdlib `net/http` + SQLite）、Vue 3.5 `<script setup lang="ts">`、daisyUI 5.8。验证：`go test ./...`、`cd web && npm run build`、`.superpowers/verify` 的浏览器脚本。

**Spec:** `docs/superpowers/specs/2026-11-27-point-item-child-scope-design.md`（下文"设计文档"；每个任务开工前读它标注的小节）。

## Global Constraints

- 语义铁律：**一个积分项没有任何 `reward_item_children` 行 = 所有孩子；有行 = 恰好这些孩子。** 不加 `scope_all` 之类标志列。
- 停用孩子**不得**级联清理关联行：不改 `handleChildUpdate`，迁移不加触发器 / 外键级联（设计文档 §4.1、§0.2）。
- `handleEarning` / `handleGuide` 只读内存快照，范围判定不得新增磁盘查询。
- `childIds` 的 JSON 约定（§4.3）：响应里"所有孩子"必须是 `[]` 不是 `null`；`PATCH` 省略或 `null` = 不动，`[]` = 改为所有孩子，非空 = 恰好这些孩子。
- 校验 `childIds`：每个 id 必须是本家庭 `role='child'` 且 `active=1` 的用户，否则 **422 `child_not_found`**；非法请求不得半写。登记不适用项的错误码逐字为 **`item_not_applicable`**（422）。
- 前端 chip 的类逐字照抄登记页 `EarnView.vue:142-148`：`btn rounded-full max-lg:btn-lg`；选中 `btn-primary border-primary/25 bg-primary/10 text-brand-ink hover:border-primary/25 hover:bg-primary/15 hover:text-brand-ink`；未选 `btn-soft`；带 `aria-pressed`。
- 文案逐字：chip 首颗 `所有孩子`；行内 ghost 按钮 `适用孩子`；弹窗标题 `适用孩子`，按钮 `取消` / `保存`；登记页空状态 `「{昵称}」暂无可登记的积分项`；范围标签里已停用的孩子名后缀 `（已停用）`。
- 保存只提交启用中的孩子；指定模式零选中时"添加" / "保存"禁用（不得把空集当成"所有孩子"）。
- 显隐规则（§4.4）：添加栏 chip 排仅当启用中的孩子 ≥ 2 时显示；范围标签与"适用孩子"按钮当 启用中的孩子 ≥ 2 **或** 列表里存在任一指定模式（`childIds.length > 0`）的项 时显示。
- 每个后端任务结束 `go test ./...` 全绿；每个前端任务结束 `cd web && npm run build` 退出码 0。
- 不新增依赖、不加测试框架；不改 `useDialogs.ts`、`api.ts`、`handleChildUpdate`、`bootstrap_handlers.go`、`internal/seed`。

## 本机环境（每个任务都必须遵守）

- **工作目录**：全程在工作树 `$WT` 内（由 superpowers:using-git-worktrees 创建，分支 `feat-item-child-scope`）；不要 `cd` 到主检出 `D:/Ryan/Code/good-habit`。git 命令只对工作树执行，每条保持简单。
- **Go 工具链**：`C:/Users/Ryan/sdk/go1.27.4/bin/go.exe`（PATH 上的 `go` 是 1.20，太旧）。下文 `go test` 均指它。
- **不要运行 `build.sh` / `build.ps1`、`npm ci`**；不要启动 vite dev server。
- **8080 是用户自己的 nginx**，绝不能占用或结束；验证一律用 `127.0.0.1:18080`。**仓库根目录的 `good-habit.db` 是用户的开发库**，不要碰。
- `VERIFY=D:/Ryan/Code/good-habit/.superpowers/verify`（主检出里的验证工具，不入库；用法见其 README）。
- **browser-use 连的是用户正在用的真实 Chrome**：只新开一个标签页、用完关掉（`base.py` 已这样做）；不要 `activate_tab`；给它长超时，绝不中途杀掉；出现 "Allow remote debugging?" 就停下来请用户点允许。（**迁移注记 2026-11-27**：browser-use 已卸载，现改用 chrome-devtools MCP，方法见 `.superpowers/verify/README.md`。）
- `/api/auth/*` 每个 IP 每分钟 10 次：检查脚本里的注册 / 登录次数要少。
- **提交**：`<type>(<scope>): <中文标题>`，正文一句话，末尾一行 `Co-Authored-By: Claude Code <noreply@anthropic.com>`；只 `git add` 本任务列出的文件；不推送、不合并、不改 `main`。

## 浏览器验证的固定流程（Task 4–6 共用）

- [ ] **开工检查**：`git status --short` 为空；`netstat -ano | grep ':18080'` 无监听。
- [ ] **起服务**：作为后台任务运行 `bash $VERIFY/serve.sh "$WT" 18080 /tmp/gh-verify build`；`curl -s http://127.0.0.1:18080/healthz` 返回 `ok` 后，`node $VERIFY/seed.mjs http://127.0.0.1:18080 /tmp/gh-verify`，再 `mkdir -p /tmp/gh-verify/shots`。种子家庭：家长 `parent`，孩子 **小明**、**小雨**（会话在 `sessions.json` 的 `parent` / `ming` / `yu`），11 个所有孩子模式的积分项。
- [ ] **写检查**：`cp $VERIFY/base.py $VERIFY/check.py`，Read 一次，再用 Edit 把本任务的断言粘到 `# <<< 断言结束` 的上一行。
- [ ] **跑出红**：`SHOTS="C:/Users/Ryan/AppData/Local/Temp/gh-verify/shots" SESS="C:/Users/Ryan/AppData/Local/Temp/gh-verify/sessions.json" browser-use < $VERIFY/check.py`。新断言此时应当失败；记下 `RESULT:` 行。（**迁移注记 2026-11-27**：browser-use 已卸载，现用 chrome-devtools MCP 执行断言清单，方法见 `.superpowers/verify/README.md`。）
- [ ] **实现**：按任务里"要做的事"改代码。
- [ ] **重建重启**：TaskStop 结束 serve.sh 的后台任务 → `bash $VERIFY/stop.sh 18080` → `rm -f /tmp/gh-verify/preview.db*`（换一个空库，否则 seed 会因账号已存在而失败）→ 同一条 serve.sh 命令（`build`）→ `healthz` → `seed.mjs`。
- [ ] **跑出绿**：再跑一次检查，最后一行必须是 `RESULT: PASS (n passed, 0 failed)`；然后用 Read 逐张查看本任务截的图（`C:/Users/Ryan/AppData/Local/Temp/gh-verify/shots/<名字>.png`）。断言通过但截图不对，同样算没完成。
- [ ] **收尾**：TaskStop → `bash $VERIFY/stop.sh 18080` → `rm -rf /tmp/gh-verify` → `cd "$WT/web" && npm run build` 再确认退出码 0 → `git status --short` 只列出本任务的文件 → 提交。

## Review Focus

设计文档隐含、但最容易被真人用户撞到、而没有页面单独为它建流程的五种情况。每一条都已在对应任务的检查里钉死：

1. **孩子昵称很长**：chip 在按钮内截断、范围标签单行省略，360px 宽无横向滚动。（Task 4 检查）
2. **`childIds` 传了 `null` 或含重复 id**：`null` 视为未传、不清空范围；重复 id 去重后保存。（Task 2 测试）
3. **指定了范围的积分项被停用再启用**：范围原样保留，重新启用后仍只对范围内的孩子可登记。（Task 3 测试）
4. **已软删除（停用）的积分项**：管理列表里仍带着它的 `childIds`，否则重新启用后标签会错。（Task 2 测试）
5. **弹窗点"保存"时接口失败**（例如另一个标签页刚停用了某个孩子）：弹窗不关、错误可见、范围不被半改。（Task 5 检查；后端侧"非法不半写"见 Task 2 测试）

---

### Task 1: 迁移 0002 与内存快照 `ChildIDs`

**Files:**
- Create: `migrations/0002_reward_item_children.sql`
- Modify: `internal/store/store.go:11-16`
- Modify: `internal/store/load.go:171-211`（`loadFamily`）
- Test: `internal/db/migrate_test.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces: 表 `reward_item_children(reward_item_id, child_id)`，`PRIMARY KEY (reward_item_id, child_id)`，索引 `idx_reward_item_children_child (child_id)`。
- Produces: `store.RewardItem` 新增字段 `ChildIDs []int64`（nil / 空 = 所有孩子）。
- Produces: `func (it RewardItem) AppliesTo(childID int64) bool`——`len(it.ChildIDs) == 0` 或包含该 id。
- Produces: `loadFamily` 把本家庭**启用中**积分项的关联行按项填进 `ChildIDs`，每个 `ChildIDs` 内按 `child_id` 升序。

- [ ] **Step 1: 写失败的迁移测试**（设计文档 §4.1、§6）

  `internal/db/migrate_test.go`：`TestMigrateFresh` 的表名列表加入 `"reward_item_children"`；`TestMigrateIdempotent` 的期望行数 `1` 改为 `2`；新增：

  ```go
  // TestMigrateFromV1：线上库停在 0001 时，升级必须原地补上 0002。
  func TestMigrateFromV1(t *testing.T) {
  	dbs := open(t)
  	v1, err := migrations.FS.ReadFile("0001_init.sql")
  	if err != nil {
  		t.Fatalf("读 0001: %v", err)
  	}
  	if err := dbs.Migrate(fstest.MapFS{"0001_init.sql": &fstest.MapFile{Data: v1}}); err != nil {
  		t.Fatalf("迁到 0001: %v", err)
  	}
  	if err := dbs.Migrate(migrations.FS); err != nil {
  		t.Fatalf("升级到 0002: %v", err)
  	}
  	var n int64
  	dbs.Read.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n)
  	if n != 2 {
  		t.Errorf("schema_migrations 行数 = %d, want 2", n)
  	}
  	var name string
  	if err := dbs.Read.QueryRow(
  		"SELECT name FROM sqlite_master WHERE type='table' AND name='reward_item_children'",
  	).Scan(&name); err != nil {
  		t.Errorf("reward_item_children 应已创建: %v", err)
  	}
  }
  ```

  （`Migrate` 的形参是 `fs.FS`，`fstest.MapFS` 可直接传入；import 加 `testing/fstest`。）

- [ ] **Step 2: 写失败的快照测试**

  `internal/store/store_test.go` 的 `seed` 里，A 家积分项循环改为拿到 `LastInsertId`，并给 **X2** 插一行关联（只给娃A）；X1、X3、B 项不写关联行：

  ```go
  res, err := tx.Exec(
  	"INSERT INTO reward_items (family_id, title, points, active, sort, created_at) VALUES (?,?,?,?,?,?)",
  	f.aFam, it.title, it.points, it.active, it.sort, now)
  if err != nil {
  	return err
  }
  // X2 只给娃A；其余不写关联行 = 所有孩子
  if it.title == "X2" {
  	id, _ := res.LastInsertId()
  	if _, err := tx.Exec("INSERT INTO reward_item_children (reward_item_id, child_id) VALUES (?,?)", id, f.aChild); err != nil {
  		return err
  	}
  }
  ```

  `TestLoadAllRebuild` 末尾追加：

  ```go
  if len(fc.Items[0].ChildIDs) != 1 || fc.Items[0].ChildIDs[0] != f.aChild {
  	t.Errorf("X2 的 ChildIDs = %v, want [%d]", fc.Items[0].ChildIDs, f.aChild)
  }
  if len(fc.Items[1].ChildIDs) != 0 {
  	t.Errorf("X1 应为所有孩子（ChildIDs 为空）: %v", fc.Items[1].ChildIDs)
  }
  ```

  新增纯函数测试：

  ```go
  func TestRewardItemAppliesTo(t *testing.T) {
  	all := RewardItem{ID: 1}
  	if !all.AppliesTo(42) || !all.AppliesTo(7) {
  		t.Error("未指定孩子应对所有人生效")
  	}
  	scoped := RewardItem{ID: 2, ChildIDs: []int64{7, 9}}
  	if !scoped.AppliesTo(7) || !scoped.AppliesTo(9) {
  		t.Error("指定孩子应生效")
  	}
  	if scoped.AppliesTo(8) {
  		t.Error("未指定的孩子不应生效")
  	}
  }
  ```

- [ ] **Step 3: 跑测试确认失败**

  Run: `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./internal/db/ ./internal/store/`
  Expected: 编译失败（`ChildIDs` / `AppliesTo` 未定义、`it.ChildIDs` 不存在）或 `TestMigrateFromV1` 失败。

- [ ] **Step 4: 写迁移文件**

  `migrations/0002_reward_item_children.sql`：

  ```sql
  -- 0002_reward_item_children.sql：积分项适用孩子（设计文档 2026-11-27-point-item-child-scope-design.md §4.1）。
  -- 一个积分项没有任何关联行 = 对所有孩子生效；有行 = 恰好对这些孩子生效。
  -- 停用孩子不级联清理本表（否则"只给这个孩子"的项会悄悄变成"所有孩子"）。

  CREATE TABLE reward_item_children (
    reward_item_id INTEGER NOT NULL REFERENCES reward_items(id),
    child_id       INTEGER NOT NULL REFERENCES users(id),
    PRIMARY KEY (reward_item_id, child_id)
  );
  CREATE INDEX idx_reward_item_children_child ON reward_item_children(child_id);
  ```

- [ ] **Step 5: 实现 `RewardItem.ChildIDs` 与 `AppliesTo`**

  `internal/store/store.go`：`RewardItem` 加 `ChildIDs []int64`，结构体注释补一句"ChildIDs 为空 = 对所有孩子生效"；新增方法：

  ```go
  // AppliesTo 报告该项能否记给 childID：未指定孩子（ChildIDs 为空）对所有孩子生效。
  func (it RewardItem) AppliesTo(childID int64) bool
  ```

  线性扫描 `ChildIDs` 即可（孩子数是个位数）。

- [ ] **Step 6: `loadFamily` 加载关联行**

  `internal/store/load.go` 的 `loadFamily`，在积分项循环 `rows.Close()` 之后、奖品查询之前，加一条查询并把结果按 `ID` 回填到 `fc.Items[i].ChildIDs`：

  ```sql
  SELECT ric.reward_item_id, ric.child_id
  FROM reward_item_children ric
  JOIN reward_items ri ON ri.id = ric.reward_item_id
  WHERE ri.family_id=? AND ri.active=1
  ORDER BY ric.reward_item_id, ric.child_id
  ```

  先读进 `map[int64][]int64`，再遍历 `fc.Items` 赋值；错误用 `fmt.Errorf("查家庭 %d 积分项适用孩子: %w", familyID, err)` 包装，与同文件风格一致。

- [ ] **Step 7: 跑测试确认通过**

  Run: `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./...`
  Expected: 全部 `ok`。

- [ ] **Step 8: Commit**

  ```bash
  git add migrations/0002_reward_item_children.sql internal/store/store.go internal/store/load.go internal/db/migrate_test.go internal/store/store_test.go
  git commit -m "feat(api): 积分项适用孩子——0002 关联表与内存快照 ChildIDs"
  ```

---

### Task 2: 积分项 CRUD 支持并校验 `childIds`

**Files:**
- Modify: `internal/httpapi/items_handlers.go`
- Test: `internal/httpapi/crud_test.go`

**Interfaces:**
- Consumes: Task 1 的 `store.RewardItem.ChildIDs`（本任务不读它，Task 3 读）。
- Produces: `itemView` 新增 `ChildIDs []int64 \`json:"childIds"\``，所有响应路径都保证非 nil（"所有孩子"序列化为 `[]`）。
- Produces: `POST /api/reward-items` body 新增 `ChildIDs []int64 \`json:"childIds"\``；`PATCH /api/reward-items/{id}` body 新增 `ChildIDs *[]int64 \`json:"childIds"\``（指针区分"没传"与"传了空"）。
- Produces（包内助手，Task 3 的测试不直接用，但同一文件内复用）:
  - `func (s *server) resolveChildIDs(familyID int64, ids []int64) ([]int64, *APIError)`——去重并校验；nil / 空 → 返回 `[]int64{}`；有非法 id → `&APIError{Status: 422, Code: "child_not_found", Msg: "孩子不存在或已停用"}`。
  - `func (s *server) childIDsByItem(itemIDs []int64) (map[int64][]int64, error)`——一次查询 `SELECT reward_item_id, child_id FROM reward_item_children WHERE reward_item_id IN (…) ORDER BY reward_item_id, child_id`；入参为空返回空 map 且不查库。
  - `func setItemChildIDs(tx *sql.Tx, itemID int64, ids []int64) error`——事务内 `DELETE FROM reward_item_children WHERE reward_item_id=?` 后逐个 INSERT（空 = 删光）。
- Produces（测试助手，供 Task 3 的测试使用）: `func mkScopedItem(t *testing.T, h http.Handler, pc *http.Cookie, title string, points int64, childIDs ...int64) itemInfo`，放在 `crud_test.go`。

- [ ] **Step 1: 写失败的测试**

  `internal/httpapi/crud_test.go`：`itemResp` 加 `ChildIDs []int64 \`json:"childIds"\``；`TestRewardItemCRUD` 创建后的断言条件追加 `|| item.ChildIDs == nil`、消息改为含 `childIds 应为 []`。新增助手与测试：

  ```go
  // mkScopedItem 建一个指定孩子的积分项（不传 childIDs 即所有孩子）。
  func mkScopedItem(t *testing.T, h http.Handler, pc *http.Cookie, title string, points int64, childIDs ...int64) itemInfo {
  	t.Helper()
  	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
  		"title": title, "points": points, "childIds": childIDs,
  	}, pc)
  	if rr.Code != 200 {
  		t.Fatalf("建指定范围积分项 = %d %s", rr.Code, rr.Body.String())
  	}
  	var it itemInfo
  	json.Unmarshal(rr.Body.Bytes(), &it)
  	return it
  }

  func TestRewardItemChildScope(t *testing.T) {
  	deps, svc := newDeps(t)
  	h := New(deps)
  	p := regParent(t, svc, "scope01")
  	pc := parentCookie(p.Token)
  	a := mkChild(t, h, pc, "scope01a", "甲")
  	b := mkChild(t, h, pc, "scope01b", "乙")

  	var item itemResp
  	// 省略 childIds → 非.nil 空切片（JSON 里是 []，不是 null）
  	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{"title": "默认", "points": 5}, pc)
  	json.Unmarshal(rr.Body.Bytes(), &item)
  	if item.ChildIDs == nil || len(item.ChildIDs) != 0 {
  		t.Fatalf("省略 childIds: %#v, want 非-nil 空切片", item.ChildIDs)
  	}

  	// 指定给甲
  	rr = doJSON(h, "POST", "/api/reward-items", map[string]any{
  		"title": "练琴", "points": 5, "childIds": []int64{a.ID},
  	}, pc)
  	if rr.Code != 200 {
  		t.Fatalf("create = %d %s", rr.Code, rr.Body.String())
  	}
  	json.Unmarshal(rr.Body.Bytes(), &item)
  	if len(item.ChildIDs) != 1 || item.ChildIDs[0] != a.ID {
  		t.Fatalf("create childIds = %v, want [%d]", item.ChildIDs, a.ID)
  	}

  	list := func() []itemResp {
  		rr := doJSON(h, "GET", "/api/reward-items", nil, pc)
  		var l struct {
  			Items []itemResp `json:"items"`
  		}
  		json.Unmarshal(rr.Body.Bytes(), &l)
  		return l.Items
  	}
  	byID := func(id int64) itemResp {
  		for _, it := range list() {
  			if it.ID == id {
  				return it
  			}
  		}
  		t.Fatalf("列表里没有项 %d", id)
  		return itemResp{}
  	}
  	if got := byID(item.ID).ChildIDs; len(got) != 1 || got[0] != a.ID {
  		t.Errorf("列表 childIds = %v, want [%d]", got, a.ID)
  	}

  	// PATCH 省略 / null → 不动
  	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"points": 8}, pc)
  	json.Unmarshal(rr.Body.Bytes(), &item)
  	if item.Points != 8 || len(item.ChildIDs) != 1 || item.ChildIDs[0] != a.ID {
  		t.Errorf("省略 childIds 的 PATCH 不应改范围: %+v", item)
  	}
  	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": nil}, pc)
  	json.Unmarshal(rr.Body.Bytes(), &item)
  	if rr.Code != 200 || len(item.ChildIDs) != 1 || item.ChildIDs[0] != a.ID {
  		t.Errorf("childIds:null 应视为未传: code=%d %+v", rr.Code, item)
  	}

  	// 重复 id 去重
  	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{
  		"childIds": []int64{a.ID, b.ID, a.ID},
  	}, pc)
  	json.Unmarshal(rr.Body.Bytes(), &item)
  	if len(item.ChildIDs) != 2 || item.ChildIDs[0] != a.ID || item.ChildIDs[1] != b.ID {
  		t.Errorf("去重后 childIds = %v, want [%d %d]", item.ChildIDs, a.ID, b.ID)
  	}

  	// 非法 id → 422 child_not_found，且不半写
  	other := regParent(t, svc, "scope01x")
  	oc := mkChild(t, h, parentCookie(other.Token), "scope01xc", "丙")
  	off := mkChild(t, h, pc, "scope01d", "丁")
  	doJSON(h, "PATCH", apiPath("children", off.ID), map[string]any{"active": false}, pc)
  	for name, bad := range map[string][]int64{
  		"他家孩子": {oc.ID}, "家长本人": {p.UserID}, "不存在": {999999}, "已停用": {off.ID, a.ID},
  	} {
  		rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": bad}, pc)
  		if rr.Code != 422 || errCode(t, rr) != "child_not_found" {
  			t.Errorf("%s: = %d %s, want 422 child_not_found", name, rr.Code, rr.Body.String())
  		}
  	}
  	if got := byID(item.ID).ChildIDs; len(got) != 2 {
  		t.Errorf("非法 PATCH 后范围被改了: %v", got)
  	}
  	if rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
  		"title": "坏", "points": 1, "childIds": []int64{999999},
  	}, pc); rr.Code != 422 || errCode(t, rr) != "child_not_found" {
  		t.Errorf("create 非法 childIds = %d %s, want 422 child_not_found", rr.Code, rr.Body.String())
  	}

  	// 跨家庭：B 家长带 childIds PATCH A 家的项 → 404
  	if rr := doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": []int64{}}, parentCookie(other.Token)); rr.Code != 404 {
  		t.Errorf("跨家庭 PATCH = %d, want 404", rr.Code)
  	}

  	// 软删除后列表仍带 childIds
  	doJSON(h, "DELETE", apiPath("reward-items", item.ID), nil, pc)
  	if got := byID(item.ID); got.Active || len(got.ChildIDs) != 2 {
  		t.Errorf("软删除后列表项不符: %+v", got)
  	}

  	// [] → 所有孩子
  	doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"active": true}, pc)
  	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": []int64{}}, pc)
  	json.Unmarshal(rr.Body.Bytes(), &item)
  	if rr.Code != 200 || item.ChildIDs == nil || len(item.ChildIDs) != 0 {
  		t.Errorf("childIds:[] 应改为所有孩子（非-nil 空切片）: code=%d %#v", rr.Code, item.ChildIDs)
  	}
  }
  ```

- [ ] **Step 2: 跑测试确认失败**

  Run: `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./internal/httpapi/ -run 'TestRewardItem'`
  Expected: 编译失败（`itemResp` 没有 `ChildIDs`）。

- [ ] **Step 3: 实现 `itemView` 与三个助手**（设计文档 §4.3）

  `internal/httpapi/items_handlers.go`：按 Interfaces 块加字段与三个函数。`resolveChildIDs` 对去重后的每个 id 跑 `SELECT 1 FROM users WHERE id=? AND family_id=? AND role='child' AND active=1`（与 `childOf` 同口径，复用 `s.deps.DBs.Read`；其他错误返回 500 `internal`）。`childIDsByItem` 用 `strings.Repeat("?,", n-1)+"?"` 拼 `IN`。

- [ ] **Step 4: 接进四个 handler**

  - `handleItemsList`：收集完全部 `itemView` 后，取 id 列表调 `childIDsByItem`，逐个赋值 `v.ChildIDs = m[v.ID]`，为 nil 时赋 `[]int64{}`。
  - `handleItemCreate`：body 加 `ChildIDs []int64`；分值校验之后 `childIDs, apiErr := s.resolveChildIDs(sess.FamilyID, body.ChildIDs)`，有错 `s.writeAPIErr` 返回；事务里 `LastInsertId` 之后 `setItemChildIDs(tx, id, childIDs)`。
  - `handleItemUpdate`：body 加 `ChildIDs *[]int64`；在 `owned()` 校验**之后**（跨家庭先 404）、事务之前，`body.ChildIDs != nil` 时调 `resolveChildIDs`；事务里同样条件下调 `setItemChildIDs(tx, id, childIDs)`。
  - `writeItemView`：扫描后 `m, err := s.childIDsByItem([]int64{id})`，赋值，nil 补 `[]int64{}`。
  - 函数注释顺手补上"适用孩子"。

- [ ] **Step 5: 跑测试确认通过**

  Run: `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./...`
  Expected: 全部 `ok`。

- [ ] **Step 6: Commit**

  ```bash
  git add internal/httpapi/items_handlers.go internal/httpapi/crud_test.go
  git commit -m "feat(api): 积分项 CRUD 支持并校验 childIds"
  ```

---

### Task 3: 登记与得分指南按范围过滤

**Files:**
- Modify: `internal/httpapi/earnings_handlers.go:43-60`（`handleEarning`）
- Modify: `internal/httpapi/read_handlers.go:17-26`（`handleGuide`）
- Modify: `docs/superpowers/specs/2026-11-25-good-habit-design.md`（§4.3 与数据模型一节）
- Test: `internal/httpapi/earnings_handlers_test.go`
- Test: `internal/httpapi/read_handlers_test.go`

**Interfaces:**
- Consumes: Task 1 的 `store.RewardItem.AppliesTo`、快照里的 `ChildIDs`；Task 2 的 `mkScopedItem` 测试助手与 `PATCH {childIds}`。
- Produces: `POST /api/earnings` 对不在项范围内的孩子返回 422 `item_not_applicable`；`GET /api/guide` 只返回 `AppliesTo(sess.UserID)` 的项。

- [ ] **Step 1: 写失败的测试**（设计文档 §4.3、§4.7、§6）

  `internal/httpapi/earnings_handlers_test.go` 追加两个测试：

  ```go
  func TestEarnItemScope(t *testing.T) {
  	deps, w := newDepsWithWriter(t)
  	h := New(deps)
  	p := regParent(t, deps.Auth, "scoper1")
  	pc := parentCookie(p.Token)
  	a := mkChild(t, h, pc, "scoper1a", "甲")
  	b := mkChild(t, h, pc, "scoper1b", "乙")

  	everyone := mkItem(t, h, pc, "大家的", 3)
  	onlyA := mkScopedItem(t, h, pc, "只给甲", 5, a.ID)

  	earn := func(childID, itemID int64) *httptest.ResponseRecorder {
  		return doJSON(h, "POST", "/api/earnings", map[string]any{
  			"childId": childID, "rewardItemId": itemID,
  		}, pc)
  	}
  	if rr := earn(b.ID, everyone.ID); rr.Code != 200 {
  		t.Errorf("所有孩子模式给乙记 = %d %s", rr.Code, rr.Body.String())
  	}
  	if rr := earn(a.ID, onlyA.ID); rr.Code != 200 {
  		t.Errorf("指定给甲、给甲记 = %d %s", rr.Code, rr.Body.String())
  	}
  	if rr := earn(b.ID, onlyA.ID); rr.Code != 422 || errCode(t, rr) != "item_not_applicable" {
  		t.Errorf("指定给甲、给乙记 = %d %s, want 422 item_not_applicable", rr.Code, rr.Body.String())
  	}
  	if got := deps.Store.Child(b.ID).Bal(); got != 3 {
  		t.Errorf("乙余额 = %d, want 3（被拒那笔不得入账）", got)
  	}

  	// 改范围后紧接着登记：快照必须已刷新
  	doJSON(h, "PATCH", apiPath("reward-items", onlyA.ID), map[string]any{"childIds": []int64{b.ID}}, pc)
  	if rr := earn(a.ID, onlyA.ID); rr.Code != 422 {
  		t.Errorf("改成只乙后给甲记 = %d, want 422", rr.Code)
  	}
  	if rr := earn(b.ID, onlyA.ID); rr.Code != 200 {
  		t.Errorf("改成只乙后给乙记 = %d, want 200", rr.Code)
  	}

  	// 停用再启用：范围保留，仍按范围生效
  	doJSON(h, "PATCH", apiPath("reward-items", onlyA.ID), map[string]any{"active": false}, pc)
  	if rr := earn(b.ID, onlyA.ID); rr.Code != 422 || errCode(t, rr) != "item_inactive" {
  		t.Errorf("停用项登记 = %d %s, want 422 item_inactive", rr.Code, rr.Body.String())
  	}
  	doJSON(h, "PATCH", apiPath("reward-items", onlyA.ID), map[string]any{"active": true}, pc)
  	if rr := earn(a.ID, onlyA.ID); rr.Code != 422 || errCode(t, rr) != "item_not_applicable" {
  		t.Errorf("重新启用后给甲记 = %d %s, want 422 item_not_applicable（范围应保留）", rr.Code, rr.Body.String())
  	}

  	// 落盘终态：甲 5（1 笔），乙 3+5（2 笔）；被拒的都不落盘
  	if err := w.Stop(); err != nil {
  		t.Fatalf("writer.Stop: %v", err)
  	}
  	for _, c := range []struct {
  		id         int64
  		sum, count int64
  	}{{a.ID, 5, 1}, {b.ID, 8, 2}} {
  		var sum, cnt int64
  		deps.DBs.Read.QueryRow(
  			"SELECT COALESCE(SUM(delta),0), COUNT(*) FROM ledger WHERE child_id=?", c.id).Scan(&sum, &cnt)
  		if sum != c.sum || cnt != c.count {
  			t.Errorf("孩子 %d 落盘 SUM=%d COUNT=%d, want %d/%d", c.id, sum, cnt, c.sum, c.count)
  		}
  	}
  }

  func TestEarnScopeSurvivesChildReactivation(t *testing.T) {
  	deps, _ := newDepsWithWriter(t)
  	h := New(deps)
  	p := regParent(t, deps.Auth, "scoper2")
  	pc := parentCookie(p.Token)
  	a := mkChild(t, h, pc, "scoper2a", "甲")
  	b := mkChild(t, h, pc, "scoper2b", "乙")

  	keep := mkScopedItem(t, h, pc, "没动过", 2, a.ID, b.ID)
  	touched := mkScopedItem(t, h, pc, "停用期间保存过", 2, a.ID, b.ID)

  	// 停用乙：关联行必须原样保留
  	doJSON(h, "PATCH", apiPath("children", b.ID), map[string]any{"active": false}, pc)
  	var n int64
  	deps.DBs.Read.QueryRow("SELECT COUNT(*) FROM reward_item_children WHERE child_id=?", b.ID).Scan(&n)
  	if n != 2 {
  		t.Fatalf("停用孩子后关联行 = %d, want 2（不得级联清理）", n)
  	}

  	// 停用期间保存「停用期间保存过」：范围只剩甲
  	doJSON(h, "PATCH", apiPath("reward-items", touched.ID), map[string]any{"childIds": []int64{a.ID}}, pc)

  	// 恢复乙
  	doJSON(h, "PATCH", apiPath("children", b.ID), map[string]any{"active": true}, pc)

  	earn := func(childID, itemID int64) int {
  		return doJSON(h, "POST", "/api/earnings", map[string]any{
  			"childId": childID, "rewardItemId": itemID,
  		}, pc).Code
  	}
  	if got := earn(b.ID, keep.ID); got != 200 {
  		t.Errorf("没动过的项：乙恢复后应重新生效 = %d, want 200", got)
  	}
  	if got := earn(b.ID, touched.ID); got != 422 {
  		t.Errorf("保存过的项：乙恢复后不应生效 = %d, want 422", got)
  	}
  }
  ```

  `internal/httpapi/read_handlers_test.go` 追加（import 加 `"net/http"`、`"slices"`）：

  ```go
  func TestGuideScopedByChild(t *testing.T) {
  	deps, _ := newDepsWithWriter(t)
  	h := New(deps)
  	p := regParent(t, deps.Auth, "guide1")
  	pc := parentCookie(p.Token)
  	a := mkChild(t, h, pc, "guide1a", "甲")
  	b := mkChild(t, h, pc, "guide1b", "乙")

  	mkItem(t, h, pc, "大家的", 1)
  	mkScopedItem(t, h, pc, "只给甲", 2, a.ID)
  	mkScopedItem(t, h, pc, "只给乙", 3, b.ID)

  	titles := func(cookie *http.Cookie) []string {
  		rr := doJSON(h, "GET", "/api/guide", nil, cookie)
  		if rr.Code != 200 {
  			t.Fatalf("guide = %d %s", rr.Code, rr.Body.String())
  		}
  		var g struct {
  			Items []struct {
  				Title string `json:"title"`
  			} `json:"items"`
  		}
  		json.Unmarshal(rr.Body.Bytes(), &g)
  		out := []string{}
  		for _, it := range g.Items {
  			out = append(out, it.Title)
  		}
  		return out
  	}
  	ka := loginAs(t, h, "guide1a", "password8")
  	kb := loginAs(t, h, "guide1b", "password8")
  	if got, want := titles(ka), []string{"大家的", "只给甲"}; !slices.Equal(got, want) {
  		t.Errorf("甲的指南 = %v, want %v", got, want)
  	}
  	if got, want := titles(kb), []string{"大家的", "只给乙"}; !slices.Equal(got, want) {
  		t.Errorf("乙的指南 = %v, want %v", got, want)
  	}
  }
  ```

- [ ] **Step 2: 跑测试确认失败**

  Run: `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./internal/httpapi/ -run 'TestEarnItemScope|TestEarnScopeSurvives|TestGuideScoped'`
  Expected: FAIL——`TestEarnItemScope`（给乙记"只给甲"返回 200 而非 422）与 `TestGuideScopedByChild`（甲的指南里有"只给乙"）；`TestEarnScopeSurvivesChildReactivation` 此刻已经通过（Task 1 / 2 已保证不级联），属正常。

- [ ] **Step 3: `handleEarning` 拒绝不适用的登记**

  `internal/httpapi/earnings_handlers.go`：在 `if item == nil { … }` 整块之后、`note := strings.TrimSpace(body.Note)` 之前插入：

  ```go
  if !item.AppliesTo(body.ChildID) {
  	writeErr(w, http.StatusUnprocessableEntity, "item_not_applicable", "该积分项不适用于这个孩子")
  	return
  }
  ```

- [ ] **Step 4: `handleGuide` 按登录孩子过滤**

  `internal/httpapi/read_handlers.go` 的 `handleGuide`：循环里 `if !it.AppliesTo(sess.UserID) { continue }`。函数注释补"只含适用于该孩子的项"。

- [ ] **Step 5: 跑全部测试确认通过**

  Run: `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./...`
  Expected: 全部 `ok`。

- [ ] **Step 6: 旧设计文档加增量说明**

  `docs/superpowers/specs/2026-11-25-good-habit-design.md`：§4.3 末尾加一行引用 `> 2026-11-27 增量：积分项可指定适用孩子，见 2026-11-27-point-item-child-scope-design.md。`；数据模型一节 `reward_items` 的 DDL 块之后补上 Task 1 的 `reward_item_children` DDL（去掉注释行）。

- [ ] **Step 7: Commit**

  ```bash
  git add internal/httpapi/earnings_handlers.go internal/httpapi/read_handlers.go internal/httpapi/earnings_handlers_test.go internal/httpapi/read_handlers_test.go docs/superpowers/specs/2026-11-25-good-habit-design.md
  git commit -m "feat(api): 登记与得分指南按积分项适用孩子过滤"
  ```

---

### Task 4: 前端类型、`ChildScopeChips` 与管理页添加栏

**Files:**
- Modify: `web/src/types.ts:82-88`
- Create: `web/src/components/ChildScopeChips.vue`
- Modify: `web/src/views/parent/ItemsView.vue`

**Interfaces:**
- Produces: `types.ts` 新增 `export type ItemScope = 'all' | number[]`（`'all'` = 所有孩子；`number[]` = 恰好这些孩子，`[]` = 指定模式但零选中）；`ItemView` 新增 `childIds: number[]`。
- Produces: `web/src/components/ChildScopeChips.vue`，props `{ children: ChildView[]; modelValue: ItemScope; label?: string }`（`label` 默认 `'适用孩子'`），emits `(e: 'update:modelValue', value: ItemScope)`。DOM：根 `div[role="group"][aria-label="适用孩子"]`，内含 `button.btn.rounded-full[aria-pressed]`，第一颗文字 `所有孩子`，之后按 `children` 顺序每个孩子一颗（只显示 `displayName`）。
- Produces（ItemsView 的 DOM，Task 5 的检查依赖）：列表行仍是 `.mt-6 > .surface`；范围标签是行内 `p.scope-label`；ghost 按钮顺序 改名 / 改分值 / 停用（Task 5 在"改分值"之后插入"适用孩子"）。

- [ ] **Step 1: 起服务、写检查、跑出红**（按"固定流程"；检查代码如下）

  ```python
  vp(1280, 820); login('parent'); go('/parent/items', 'light')
  wait_for("document.querySelectorAll('.mt-6 > .surface').length >= 11")
  rows = '.mt-6 > .surface'
  chips = 'div[aria-label="适用孩子"] button'
  pressed = "document.querySelector(%s).getAttribute('aria-pressed')"
  set_inputs = "(() => { const t = document.querySelector('.join input:not([type])'); t.value = %s; t.dispatchEvent(new Event('input', {bubbles: true})); const p = document.querySelector('.join input[type=number]'); p.value = %s; p.dispatchEvent(new Event('input', {bubbles: true})); return true })()"
  add_btn = "document.querySelector('.join .btn-primary')"

  # 两孩家庭：chip 排可见，默认「所有孩子」按下
  check('chip row: 所有孩子 + one per child', count(chips) == 3 and sq(text(chips)) == '所有孩子', texts(chips))
  check('default is all-mode', js(pressed % json.dumps(chips)) == 'true')
  check('all-mode rows labelled 所有孩子', all('所有孩子' in t for t in texts(rows)))
  shot('t4-items-default')

  # 点「小明」→ 指定模式、只有小明按下；再取消 → 零选中，「添加」禁用
  js("document.querySelectorAll(%s)[1].click()" % json.dumps(chips))
  wait_for("document.querySelectorAll(%s)[1].getAttribute('aria-pressed') === 'true'" % json.dumps(chips))
  check('clicking a child leaves all-mode', js(pressed % json.dumps(chips)) == 'false' and js("document.querySelectorAll(%s)[1].textContent.trim()" % json.dumps(chips)) == '小明')
  js("document.querySelectorAll(%s)[1].click()" % json.dumps(chips))
  wait_for("document.querySelectorAll(%s)[1].getAttribute('aria-pressed') === 'false'" % json.dumps(chips))
  check('untick last child stays explicit (all not pressed)', js(pressed % json.dumps(chips)) == 'false')
  js(set_inputs % (json.dumps('验证项'), json.dumps('3')))
  check('添加 disabled when explicit mode has zero ticked', js(add_btn + '.disabled'))

  # 点「所有孩子」→ 可添加；行标签 = 所有孩子；创建后 chip 复位
  js("document.querySelector(%s).click()" % json.dumps(chips))
  js(add_btn + '.click()')
  wait_for("[...document.querySelectorAll('.font-medium')].some(e => e.textContent.trim() === '验证项')")
  r = [t for t in texts(rows) if '验证项' in t]
  check('all-mode create: label 所有孩子', len(r) == 1 and '所有孩子' in r[0], r)
  check('scope resets to all after create', js(pressed % json.dumps(chips)) == 'true')

  # 指定给小明创建：行标签是孩子名，不是「所有孩子」
  js("document.querySelectorAll(%s)[1].click()" % json.dumps(chips))
  js(set_inputs % (json.dumps('只给小明'), json.dumps('4'))); js(add_btn + '.click()')
  wait_for("[...document.querySelectorAll('.font-medium')].some(e => e.textContent.trim() === '只给小明')")
  r = [t for t in texts(rows) if '只给小明' in t]
  check('scoped create: label names the child', len(r) == 1 and '小明' in r[0] and '所有孩子' not in r[0], r)
  shot('t4-items-scoped')

  # 长昵称：chip 在按钮内截断、行标签单行省略、360 宽无横向滚动（窄屏量才有意义）
  kids = {c['displayName']: c['id'] for c in api('GET', '/api/children', None, SESS['parent'])[0]['children']}
  items = api('GET', '/api/reward-items', None, SESS['parent'])[0]['items']
  long = '小雨' + '雨' * 38
  api('PATCH', '/api/children/%d' % kids['小雨'], {'displayName': long}, SESS['parent'])
  api('PATCH', '/api/reward-items/%d' % [i for i in items if i['title'] == '只给小明'][0]['id'], {'childIds': [kids['小雨']]}, SESS['parent'])
  vp(390, 844, 2, True); go('/parent/items')
  wait_for("[...document.querySelectorAll('.scope-label')].some(e => e.textContent.includes('雨雨'))"); time.sleep(0.5)
  check('long name: chip stays inside the viewport', js("(() => { const b = [...document.querySelectorAll(%s)].find(b => b.textContent.includes('雨雨')); return b.getBoundingClientRect().right <= window.innerWidth })()" % json.dumps(chips)))
  check('long name: label truncates (no wrap)', js("(() => { const e = [...document.querySelectorAll('.scope-label')].find(e => e.textContent.includes('雨雨')); return e.scrollWidth > e.clientWidth })()"))
  no_horizontal_scroll(360)
  shot('t4-long-name-360')

  # 单孩家庭：没有 chip 排、没有范围标签、行里只有 3 个按钮
  logout()
  pw = [l.split('=', 1)[1].strip() for l in open(os.path.join(os.path.dirname(os.environ['SESS']), 'creds.env')) if l.startswith('PW=')][0]
  _, solo = api('POST', '/api/auth/register', {'username': 'solo_dad' + str(int(time.time()))[-6:], 'password': pw, 'family_name': '单娃'})
  api('POST', '/api/children', {'username': 'solo_kid' + str(int(time.time()))[-6:], 'password': pw, 'displayName': '独苗'}, solo)
  api('POST', '/api/reward-items', {'title': '独苗项', 'points': 1}, solo)
  login_token(solo); go('/parent/items')
  wait_for("[...document.querySelectorAll('.font-medium')].some(e => e.textContent.trim() === '独苗项')")
  check('single-child family: no chip row', count(chips) == 0)
  check('single-child family: no scope label', count('.scope-label') == 0)
  check('single-child family: only 3 row buttons', count(rows + ' button') == 3, count(rows + ' button'))
  vp(1280, 820); shot('t4-items-single')
  ```

  预期红：从 `chip row…` 起的断言全部失败（chip 排与标签还不存在；长昵称那段的 `wait_for` 会超时）。注意本任务不用 `fail_api` 之类的 init script；`api()` 直接走 Python，不受页面影响。

- [ ] **Step 2: 类型**

  `web/src/types.ts`：在 `ItemView` 前加 `ItemScope` 类型（带注释"适用范围：'all' = 所有孩子；number[] = 恰好这些孩子，[] = 指定模式但零选中"），`ItemView` 加 `childIds: number[]`（注释"适用孩子；空数组 = 所有孩子"）。

- [ ] **Step 3: `ChildScopeChips.vue`**

  `web/src/components/ChildScopeChips.vue`（`<script setup lang="ts">`，`withDefaults` 的 `label` 默认 `'适用孩子'`）。切换逻辑只有两个函数：

  ```ts
  function pickAll(): void { emit('update:modelValue', 'all') }
  function toggle(id: number): void {
    const cur = props.modelValue
    if (cur === 'all') { emit('update:modelValue', [id]); return }
    emit('update:modelValue', cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id])
  }
  ```

  模板：`<div role="group" :aria-label="label" class="flex flex-wrap gap-2">`；"所有孩子"按钮 `:aria-pressed="modelValue === 'all'"`、`@click="pickAll"`；每个孩子 `:aria-pressed="modelValue !== 'all' && modelValue.includes(c.id)"`、`@click="toggle(c.id)"`。两处 `:class` 都按 Global Constraints 里的三个类串：`aria-pressed` 为真用选中串，否则 `'btn-soft'`（与 `EarnView.vue:143-147` 同一写法）。不带余额数字。每颗按钮加 `max-w-full`，文字包在 `<span class="truncate">` 里——`.btn` 不换行，不加这两处一个长昵称就会把 chip 排撑出 360px 视口。

- [ ] **Step 4: `ItemsView.vue` 接入添加栏与行标签**

  - 新增 `children = ref<ChildView[]>([])`、`activeChildren = computed(() => children.value.filter((c) => c.active))`、`newScope = ref<ItemScope>('all')`；`load()` 改为 `Promise.all` 同时取 `/api/children` 与 `/api/reward-items`。
  - 显隐：`showAddChips = computed(() => activeChildren.value.length >= 2)`；`showScopeControls = computed(() => showAddChips.value || items.value.some((it) => it.childIds.length > 0))`。
  - `create()`：请求体加 `childIds: newScope.value === 'all' ? [] : newScope.value`；成功后 `newScope.value = 'all'`。"添加"按钮 `:disabled` 追加 `|| (newScope !== 'all' && newScope.length === 0)`。
  - 模板：`join` 栏之下、提示文字之上，`<ChildScopeChips v-if="showAddChips" v-model="newScope" :children="activeChildren" />`。
  - 行标签：`RowCard` 默认插槽里标题 `<p>` 之下加 `<p v-if="showScopeControls" class="scope-label truncate text-xs text-base-content/50">{{ scopeLabel(it) }}</p>`。`scopeLabel(it: ItemView): string`——`childIds` 为空返回 `'所有孩子'`；否则用**完整**孩子列表（含停用）按 id 找名字，已停用的加后缀 `（已停用）`，用 `、` 连接；找不到的 id 跳过。

- [ ] **Step 5: 重建重启、跑出绿、看截图**（按"固定流程"）

  Run: 同一条 `browser-use < $VERIFY/check.py`（**迁移注记 2026-11-27**：browser-use 已卸载，现用 chrome-devtools MCP 执行本 flow 的断言清单，见 `.superpowers/verify/README.md` §5）
  Expected: `RESULT: PASS`；截图里 chip 排在添加栏下方、选中态与登记页一致，长名字那行标签单行省略，单孩家庭页面与现在完全一致。

- [ ] **Step 6: Commit**

  ```bash
  git add web/src/types.ts web/src/components/ChildScopeChips.vue web/src/views/parent/ItemsView.vue
  git commit -m "feat(web): 积分项管理页——添加栏适用孩子 chip 与范围标签"
  ```

---

### Task 5: 「适用孩子」弹窗

**Files:**
- Create: `web/src/components/ItemScopeDialog.vue`
- Modify: `web/src/views/parent/ItemsView.vue`

**Interfaces:**
- Consumes: Task 4 的 `ItemScope`、`ChildScopeChips`（`v-model` + `children`）、`scopeLabel`。
- Produces: `ItemScopeDialog.vue`，props `{ item: ItemView; children: ChildView[] }`（`children` 只含启用中的），emits `(e: 'close'): void`、`(e: 'saved'): void`。自己负责 PATCH、错误展示与"保存"禁用；父组件只负责挂载 / 卸载与刷新列表。DOM：`dialog[open]` 内 `h3` 文字 `适用孩子`，其下一个 `<p>` 显示积分项名，再下是 `ChildScopeChips`，`.modal-action` 里按钮 `取消` / `保存`，出错时 `.alert-error[role="alert"]`。
- Produces（ItemsView 的 DOM）：ghost 按钮顺序 改名 / 改分值 / **适用孩子** / 停用；"适用孩子"带 `v-if="showScopeControls"`。

- [ ] **Step 1: 起服务、写检查、跑出红**（按"固定流程"；检查代码如下）

  ```python
  vp(1280, 820); login('parent')
  rows = '.mt-6 > .surface'
  dlg = 'dialog[open]'
  chips = dlg + ' div[aria-label="适用孩子"] button'
  btn = "[...document.querySelectorAll(%s button)].find(b => b.textContent.trim() === %s)" % (json.dumps(dlg), '%s')
  open_dlg = "[...document.querySelectorAll(%s)].find(r => r.textContent.includes(%s)).querySelectorAll('button')[2].click()"
  kids = {c['displayName']: c['id'] for c in api('GET', '/api/children', None, SESS['parent'])[0]['children']}
  item = lambda title: [i for i in api('GET', '/api/reward-items', None, SESS['parent'])[0]['items'] if i['title'] == title][0]

  # 接口失败：保存不关弹窗、错误可见、范围不变（fail_api 要在 go() 之前挂；只匹配带斜杠的 PATCH 路径）
  fid = fail_api('/api/reward-items/')
  go('/parent/items'); wait_for("document.querySelectorAll(%s).length >= 11" % json.dumps(rows))
  check('4 buttons per row in 2-child family', count(rows + ' button') == 44, count(rows + ' button'))
  js(open_dlg % (json.dumps(rows), json.dumps('托管课写完作业'))); wait_for("document.querySelector('dialog[open]')")
  check('dialog title + item name', sq(text(dlg + ' h3')) == '适用孩子' and '托管课写完作业' in text(dlg))
  check('dialog opens in all-mode', js("document.querySelector(%s).getAttribute('aria-pressed')" % json.dumps(chips)) == 'true')
  js("document.querySelectorAll(%s)[2].click()" % json.dumps(chips))   # 小雨
  js(btn % "'保存'" + '.click()'); wait_for("document.querySelector('dialog[open] .alert-error')")
  check('API error: dialog stays open with alert', count('dialog[open] .alert-error') == 1)
  check('API error: scope unchanged', item('托管课写完作业')['childIds'] == [])
  remove_init_script(fid)
  js(btn % "'取消'" + '.click()'); wait_for("!document.querySelector('dialog[open]')")

  # 取消 / Esc 不改范围；保存才改
  go('/parent/items'); wait_for("document.querySelectorAll(%s).length >= 11" % json.dumps(rows))
  js(open_dlg % (json.dumps(rows), json.dumps('托管课写完作业'))); wait_for("document.querySelector('dialog[open]')")
  js("document.querySelectorAll(%s)[2].click()" % json.dumps(chips))
  js(btn % "'取消'" + '.click()'); wait_for("!document.querySelector('dialog[open]')")
  check('cancel keeps scope', item('托管课写完作业')['childIds'] == [])
  js(open_dlg % (json.dumps(rows), json.dumps('托管课写完作业'))); wait_for("document.querySelector('dialog[open]')")
  js("document.querySelectorAll(%s)[2].click()" % json.dumps(chips))
  check('保存 enabled with one ticked', not js(btn % "'保存'" + '.disabled'))
  js(btn % "'保存'" + '.click()')
  wait_for("!document.querySelector('dialog[open]') && [...document.querySelectorAll(%s)].some(r => r.textContent.includes('托管课写完作业') && r.textContent.includes('小雨'))" % json.dumps(rows))
  check('save PATCHes childIds', item('托管课写完作业')['childIds'] == [kids['小雨']], item('托管课写完作业')['childIds'])
  shot('t5-dialog-saved')

  # 取消最后一个 → 保存禁用、所有孩子不按下；点「所有孩子」恢复可用；Esc 关闭不改范围
  js(open_dlg % (json.dumps(rows), json.dumps('托管课写完作业'))); wait_for("document.querySelector('dialog[open]')")
  js("document.querySelectorAll(%s)[2].click()" % json.dumps(chips))   # 取消小雨
  check('保存 disabled after unticking the last child', js(btn % "'保存'" + '.disabled'))
  check('all chip stays unpressed', js("document.querySelector(%s).getAttribute('aria-pressed')" % json.dumps(chips)) == 'false')
  js("document.querySelector(%s).click()" % json.dumps(chips))
  check('所有孩子 re-enables 保存', not js(btn % "'保存'" + '.disabled'))
  js("document.querySelector('dialog[open]').dispatchEvent(new Event('cancel', {cancelable: true}))")
  wait_for("!document.querySelector('dialog[open]')")
  check('Esc keeps scope', item('托管课写完作业')['childIds'] == [kids['小雨']])

  # 孤儿项：把「额外作业」指定给小雨，再停用小雨 → 只剩一个启用孩子，标签与按钮仍在
  api('PATCH', '/api/reward-items/%d' % item('额外作业')['id'], {'childIds': [kids['小雨']]}, SESS['parent'])
  api('PATCH', '/api/children/%d' % kids['小雨'], {'active': False}, SESS['parent'])
  go('/parent/items'); wait_for("[...document.querySelectorAll(%s)].some(r => r.textContent.includes('额外作业'))" % json.dumps(rows))
  r = [t for t in texts(rows) if '额外作业' in t][0]
  check('orphan item: label names the inactive child', '小雨' in r and '（已停用）' in r, r)
  check('orphan item: 适用孩子 button still shown', '适用孩子' in r)
  check('all-mode rows still labelled (an explicit item exists)', all('所有孩子' in t for t in texts(rows) if '额外作业' not in t and '托管课写完作业' not in t))
  shot('t5-orphan')
  js(open_dlg % (json.dumps(rows), json.dumps('额外作业'))); wait_for("document.querySelector('dialog[open]')")
  check('orphan dialog: nothing pressed, 保存 disabled', js("[...document.querySelectorAll(%s)].every(b => b.getAttribute('aria-pressed') === 'false')" % json.dumps(chips)) and js(btn % "'保存'" + '.disabled'))
  js("document.querySelector(%s).click()" % json.dumps(chips))   # 所有孩子
  js(btn % "'保存'" + '.click()'); wait_for("!document.querySelector('dialog[open]')")
  check('orphan fixed via 所有孩子', item('额外作业')['childIds'] == [])
  ```

  预期红：`4 buttons per row` 失败（现在只有 3 个），`dialog title…` 起因 `dialog[open]` 永不出现而 `wait_for` 超时。

- [ ] **Step 2: `ItemScopeDialog.vue`**（设计文档 §4.4）

  `web/src/components/ItemScopeDialog.vue`，`<script setup lang="ts">`：

  - 草稿只含启用中的孩子：`const activeIds = new Set(props.children.map((c) => c.id))`；`const draft = ref<ItemScope>(props.item.childIds.length === 0 ? 'all' : props.item.childIds.filter((id) => activeIds.has(id)))`（孤儿项 → `[]`，即指定模式零选中）。
  - `const canSave = computed(() => draft.value === 'all' || draft.value.length > 0)`；`saving = ref(false)`、`error = ref('')`。
  - `async function save(): Promise<void>`——`if (!canSave.value || saving.value) return`；`api('PATCH', \`/api/reward-items/${props.item.id}\`, { childIds: draft.value === 'all' ? [] : draft.value })`；成功 `emit('saved')`，失败 `error.value = errMsg(e)`（弹窗不关）。
  - 模板：`<BaseDialog :label="`适用孩子 · ${item.title}`" @close="!saving && emit('close')">` 内放 `<h3 class="text-lg font-bold">适用孩子</h3>`、`<p class="text-sm text-base-content/70">{{ item.title }}</p>`、`<ChildScopeChips v-model="draft" :children="children" />`、`<div v-if="error" class="alert alert-error py-2 text-sm" role="alert">{{ error }}</div>`、`<div class="modal-action">` 取消（`btn-ghost`，`@click="emit('close')"`）与 保存（`btn-primary`，`:disabled="!canSave || saving"`，`@click="save"`）。按钮都带 `max-lg:btn-lg`，与 `PromptDialog.vue` 一致。

- [ ] **Step 3: `ItemsView.vue` 挂上弹窗**

  - `import ItemScopeDialog from '../../components/ItemScopeDialog.vue'`；`const scopeTarget = ref<ItemView | null>(null)`。
  - `#actions` 里"改分值"之后插入 `<button v-if="showScopeControls" class="btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm" @click="scopeTarget = it">适用孩子</button>`。
  - 模板末尾（`EmptyState` 之后）加 `<ItemScopeDialog v-if="scopeTarget" :item="scopeTarget" :children="activeChildren" @close="scopeTarget = null" @saved="onScopeSaved" />`；`function onScopeSaved(): void { scopeTarget.value = null; load() }`。

- [ ] **Step 4: 重建重启、跑出绿、看截图**（按"固定流程"）

  Run: 同一条 `browser-use < $VERIFY/check.py`（**迁移注记 2026-11-27**：browser-use 已卸载，现用 chrome-devtools MCP 执行本 flow 的断言清单，见 `.superpowers/verify/README.md` §5）
  Expected: `RESULT: PASS`；截图里弹窗是底部 / 居中 modal，chip 排与添加栏同款，孤儿项那行标签带"（已停用）"。

- [ ] **Step 5: Commit**

  ```bash
  git add web/src/components/ItemScopeDialog.vue web/src/views/parent/ItemsView.vue
  git commit -m "feat(web): 积分项「适用孩子」弹窗"
  ```

---

### Task 6: 登记页按选中孩子过滤

**Files:**
- Modify: `web/src/views/parent/EarnView.vue:18`（`activeItems`）、`:189-191`（空状态）

**Interfaces:**
- Consumes: `ItemView.childIds`（Task 4）。
- Produces: `activeItems` 过滤规则 `i.active && (selected === null || i.childIds.length === 0 || i.childIds.includes(selected))`；`const emptyText = computed<string>(…)`；空状态 `<p class="py-8 …">` 文字来自 `emptyText`。

- [ ] **Step 1: 起服务、写检查、跑出红**（按"固定流程"；检查代码如下）

  ```python
  vp(1280, 820); login('parent')
  tiles = 'section .grid button'
  kid_chip = "[...document.querySelectorAll('button.btn.rounded-full')].find(b => b.textContent.includes(%s))"
  kids = {c['displayName']: c['id'] for c in api('GET', '/api/children', None, SESS['parent'])[0]['children']}
  items = api('GET', '/api/reward-items', None, SESS['parent'])[0]['items']
  api('POST', '/api/reward-items', {'title': '只给小明', 'points': 1, 'childIds': [kids['小明']]}, SESS['parent'])
  api('POST', '/api/reward-items', {'title': '只给小雨', 'points': 1, 'childIds': [kids['小雨']]}, SESS['parent'])

  go('/parent/earn'); wait_for("document.querySelectorAll(%s).length > 0" % json.dumps(tiles))
  check('default child 小明: sees 只给小明, hides 只给小雨', any('只给小明' in t for t in texts(tiles)) and not any('只给小雨' in t for t in texts(tiles)), texts(tiles))
  check('all-mode item visible to 小明', any('托管课写完作业' in t for t in texts(tiles)))
  js(kid_chip % json.dumps('小雨') + '.click()')
  wait_for("[...document.querySelectorAll(%s)].some(b => b.textContent.includes('只给小雨'))" % json.dumps(tiles))
  check('switch to 小雨: sees 只给小雨, hides 只给小明', any('只给小雨' in t for t in texts(tiles)) and not any('只给小明' in t for t in texts(tiles)), texts(tiles))
  check('all-mode item visible to 小雨', any('托管课写完作业' in t for t in texts(tiles)))
  shot('t6-earn-filtered')

  # 孩子端指南：小明看得到「只给小明」，看不到「只给小雨」
  logout(); login('ming'); go('/kid/home'); wait_for("document.body.textContent.includes('只给小明')")
  check('kid guide hides sibling-only item', '只给小雨' not in text('body'))
  check('kid guide shows own + all-mode items', '只给小明' in text('body') and '托管课写完作业' in text('body'))
  shot('t6-kid-guide')

  # 空状态：把全部积分项改成只给小雨 → 小明没有任何可登记项
  logout(); login('parent')
  for i in items: api('PATCH', '/api/reward-items/%d' % i['id'], {'childIds': [kids['小雨']]}, SESS['parent'])
  api('PATCH', '/api/reward-items/%d' % [i for i in api('GET', '/api/reward-items', None, SESS['parent'])[0]['items'] if i['title'] == '只给小明'][0]['id'], {'childIds': [kids['小雨']]}, SESS['parent'])
  go('/parent/earn'); wait_for("document.querySelector('p.py-8')")
  check('empty state names the child', sq(text('p.py-8')) == '「小明」暂无可登记的积分项', text('p.py-8'))
  shot('t6-earn-empty')
  ```

  预期红：`default child 小明…` 与 `switch to 小雨…` 失败（两项都显示）；`empty state…` 失败（仍显示"还没有积分项"）。孩子端指南那条在 Task 3 之后已经通过。

- [ ] **Step 2: 实现**

  `web/src/views/parent/EarnView.vue`：
  - `activeItems` 按 Interfaces 的规则过滤（`selected.value` 为 `null` 时只按 `active`）。
  - 加 `const emptyText = computed(() => { if (!items.value.some((i) => i.active)) return '还没有积分项，先去「积分项」页添加'; const c = children.value.find((x) => x.id === selected.value); return c ? \`「${c.displayName}」暂无可登记的积分项\` : '还没有积分项，先去「积分项」页添加' })`。
  - 空状态 `<p>` 里的固定文字换成 `{{ emptyText }}`。

- [ ] **Step 3: 重建重启、跑出绿、看截图**（按"固定流程"）

  Run: 同一条 `browser-use < $VERIFY/check.py`（**迁移注记 2026-11-27**：browser-use 已卸载，现用 chrome-devtools MCP 执行本 flow 的断言清单，见 `.superpowers/verify/README.md` §5）
  Expected: `RESULT: PASS`；截图里切到小雨后右侧格子只含小雨可记的项，空状态文案带「小明」。

- [ ] **Step 4: Commit**

  ```bash
  git add web/src/views/parent/EarnView.vue
  git commit -m "feat(web): 登记页按选中孩子过滤积分项"
  ```

---

## 收尾核对（全部任务完成后）

- [ ] `cd "$WT" && C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./...` 全绿；`cd "$WT/web" && npm run build` 退出码 0。
- [ ] `git log --oneline main..HEAD` 恰好 6 个提交；`git status --short` 为空。
- [ ] 对照设计文档 §4.7 校验表逐行过一遍，确认每行都有对应测试或检查。
