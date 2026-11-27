# 好习惯 · 前端视觉工艺升级 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改识别、不改页面结构的前提下，为 `web/` 的全部 12 个路由页面补上字号/间距/表面/动效刻度，并落实用户点名的登记页改动（卡片在左、白底积分项、登记前确认）。

**Architecture:** 纯前端改动。刻度集中在 `web/src/styles/app.css`（daisyUI 主题变量 + Tailwind `@theme` token + 少量组件类），页面与组件只改模板里的类名和少量标记；新增一个 `TickNumber` 组件承担"积分数字会动"；唯一的行为变更是登记得分先确认。不新增依赖、不新增接口、不引入测试框架。

**Tech Stack:** Vue 3.5（`<script setup lang="ts">`）、vue-router 4、Tailwind CSS 4.3、daisyUI 5.8；验证用 `npm run build`、Go 二进制托管产物、本机 `browser-use` CLI。

**Spec:** `docs/superpowers/specs/2026-11-26-web-visual-upgrade-design.md`（下文简称"设计文档"；执行每个任务前必须读它标注的小节）。参考标记（已获用户确认的效果图，一次性产物，不入库）：`.superpowers/mockup-kit/pages/{kid-home,parent-overview,parent-earn}.html` 与 `.superpowers/mockup-kit/mock.source.css`。

## Global Constraints

- 不引入任何新依赖、字体文件或图片资源；不改配色、字体、插画；不改信息架构与导航顺序；未出效果图的页面只套刻度、不改布局。
- 字重只用 400 / 500 / 600 / 700；全项目不得出现 `font-extrabold`（`grep -rn "font-extrabold" web/src` 无输出）。
- 普通卡片无阴影（描边或阴影二选一）；全项目不得出现 `shadow-sm` / `shadow-md` / `shadow-lg`；阴影只用 `shadow-overlay` 与 `shadow-hero`。
- <lg 触控目标宽高都 ≥ 44px；任何页面在 360px 宽度下无横向滚动。
- `prefers-reduced-motion: reduce` 时不得有任何动画（CSS 由现有全局 kill-switch 负责；JS 驱动的数字滚动与浮动标签必须读 `matchMedia` 后跳过）。
- 大号独立数字（hero / display / stat）用比例数字，不加 `tabular-nums`；需要对齐的数字（账单增减、清单分值、选人胶囊里的余额）加 `tabular-nums`。数字用文字色，不用品牌色。
- 登记得分的确认文案逐字为：标题 `登记得分`，正文 `给 {孩子昵称} 记一笔「{积分项标题}」，+{分值} 分。`，确认按钮 `登记`，取消按钮 `取消`。
- 浮动标签文字：登记为 `+{分值}`；转盘扣分为 `−{价格}`（`−` 是 U+2212 减号，不是连字符）。
- 每个任务结束时 `cd web && npm run build` 退出码为 0。
- 不改 `WheelCanvas.vue`、`useDialogs.ts`、`useToast.ts`、`api.ts`、后端代码；不给「手动调整」加确认框；不改 `EarnView` 的 `flash()` 防抖。
- daisyUI 5.8.2 既定行为（设计文档 §3）：带 `aria-pressed="true"` 的 `.btn` 会被强制成实心外观，选中态必须用 `bg-*` / `border-*` / `text-*` 工具类覆盖；`.btn` 自带 `overflow: hidden`，浮动标签放在按钮外层的定位容器里；`.menu` 的链接要加 `content-center` 才垂直居中；`btn-soft btn-primary` 的文字要用 `text-brand-ink`。

## 本机环境（每个任务都必须遵守）

- **工作目录**：全程在 `D:/Ryan/Code/good-habit/.claude/worktrees/feat-web-visual-upgrade`（下称 `$WT`）内操作；不要 `cd` 到主检出目录 `D:/Ryan/Code/good-habit`。git 命令只对工作树执行，并且每条保持简单（隔离会话会拒绝带命令替换、`git -C` 等复杂形式）。
- **分支**：`worktree-feat-web-visual-upgrade`。每个任务结束提交一次；**不要推送、不要合并、不要改 `main`**。
- **Go 工具链**：`C:/Users/Ryan/sdk/go1.27.4/bin/go.exe`（PATH 上的 `go` 是 1.20，太旧）。
- **不要运行 `build.sh` / `build.ps1`**，不要运行 `npm ci`（依赖已装好），不要启动 vite dev server。
- **8080 端口是用户自己的 nginx**，绝不能占用或结束；验证一律用 `127.0.0.1:18080`。
- **仓库根目录的 `good-habit.db` 是用户的开发库**，不要打开、复制、删除或拿来登录。
- **浏览器**：`browser-use` 连的是用户正在使用的真实 Chrome，里面有用户的工作标签页。只新开一个标签页（`base.py` 已这样做）、用完关掉；不要调用 `activate_tab`（会把用户的浏览器切到前台）。Chrome 窗口被遮挡时 `requestAnimationFrame` 与定时器会被节流——判断"已加载"用 `wait_for(...)` 轮询，不用固定 `sleep`。
- **接口限速**：`/api/auth/*` 每个 IP 每分钟 10 次。一个验证服务实例上，检查脚本不要连续运行超过 3 次（每次会新建并登录 2 个临时孩子）；超过了就重启服务再跑。
- **写文件**：仓库内的文件用 Write / Edit 工具；PowerShell 不要用 `-ExecutionPolicy Bypass`。

## 每个任务的固定流程

`VERIFY` = `$WT/.superpowers/verify`（说明见 `$VERIFY/README.md`；该目录不入库）。

- [ ] **开工检查**：`git status --short` 为空（只剩本任务要改的文件之前的状态）；`netstat -ano | grep ':18080'` 无监听。
- [ ] **起服务（改之前，不重新构建）**：作为后台任务运行 `bash $VERIFY/serve.sh D:/Ryan/Code/good-habit/.claude/worktrees/feat-web-visual-upgrade 18080 /tmp/gh-verify nobuild`；`curl -s http://127.0.0.1:18080/healthz` 返回 `ok` 后，`node $VERIFY/seed.mjs http://127.0.0.1:18080 C:/Users/Ryan/AppData/Local/Temp/gh-verify`，再 `mkdir -p /tmp/gh-verify/shots`。
- [ ] **写检查**：`cp $VERIFY/base.py $VERIFY/check.py`，Read 一次，再用 Edit 把本任务"检查"一节的代码粘到 `# <<< 断言结束` 的上一行。
- [ ] **跑出红**：`SHOTS="C:/Users/Ryan/AppData/Local/Temp/gh-verify/shots" SESS="C:/Users/Ryan/AppData/Local/Temp/gh-verify/sessions.json" browser-use < $VERIFY/check.py`。本任务的断言此时应当失败（现有页面还没改）；把 `RESULT:` 行记下来。
- [ ] **实现**：按"要做的事"修改代码。
- [ ] **重建并重启服务**：TaskStop 结束 serve.sh 的后台任务 → `bash $VERIFY/stop.sh 18080` → 作为后台任务运行同一条 serve.sh 命令但把 `nobuild` 换成 `build` → `healthz` → `seed.mjs`（库是新的，要重新造数据）。
- [ ] **跑出绿**：再运行一次检查，最后一行必须是 `RESULT: PASS (n passed, 0 failed)`；然后用 Read 工具逐张查看本任务截的图（路径 `C:/Users/Ryan/AppData/Local/Temp/gh-verify/shots/<名字>.png`），确认与效果图一致、没有错位或溢出。断言通过但截图不对，同样算没完成。
- [ ] **收尾**：TaskStop → `bash $VERIFY/stop.sh 18080` → `rm -rf /tmp/gh-verify` → `cd $WT/web && npm run build` 再确认一次退出码 0 → `git status --short` 只列出本任务的文件 → 提交。

提交信息统一为 `feat(web): 视觉工艺升级（N/9）— <本任务标题>`，正文一句话说明改了什么，末尾一行 `Co-Authored-By: Claude Code <noreply@anthropic.com>`。只 `git add` 本任务列出的文件。

## 检查用选择器与跨任务接口

后一个任务的检查依赖前一个任务留下的这些名字，实现时必须使用：

| 名字 | 定义于 | 用途 |
|---|---|---|
| `.surface` / `.inset` / `.divided` / `.stagger` / `.field-label` / `.status-chip` / `.float-chip` / `.page-enter-active` | Task 1（`app.css`） | 所有任务的样式与检查 |
| `text-hero`（`lg:text-hero-lg`）、`text-display`、`text-stat`、`text-title`、`shadow-overlay`、`shadow-hero` | Task 1 | 字号与阴影工具类 |
| `TickNumber.vue`：props `value: number`、`enter?: boolean` | Task 4 | Task 6、Task 8 复用 |
| `TickNumber` 的 DOM：根 `<span>`（透传 class，`inline-block`）内含 `<span class="sr-only">` 与 `<span aria-hidden="true">` | Task 4 | 检查用 `.text-hero [aria-hidden="true"]` 取可见数字 |
| 布局里 `<Transition name="page" mode="out-in" appear>` 包住 `<RouterView>` 的组件 | Task 2 | 路由过渡 |
| 孩子端主数字卡 `section[aria-label="我的积分"]` | Task 4 | 检查 |
| 总览概况 `section[aria-label="概况"]`，三个格子是它的直接子元素 | Task 7 | 检查 |
| 登记页积分项：`.grid` 里每项是 `<span class="relative block">` 包一个 `button.btn` | Task 8 | 检查 |

## Review Focus

规格隐含、但最容易被真人用户撞到、而没有任务单独为它建页面的五种情况。每一条都已在对应任务的检查里钉死：

1. **系统开启了"减少动态效果"**：主数字直接显示最终值，不出现 `.float-chip`，待选横幅不再摇。（Task 4、Task 8）
2. **新孩子的余额是 0，或余额是四位数**：主数字显示 `0` / `1234`，不出错、不溢出、无横向滚动。（Task 4）
3. **孩子昵称很长**：总览卡片里名字被截断、卡片不被撑高；登记页选人胶囊不溢出。（Task 7、Task 8）
4. **接口很慢或失败**：总览在数据返回前不显示三个 0，接口失败时只显示标题、不崩溃；转盘页在数据返回前不显示 0。（Task 6、Task 7）
5. **确认框的各种退出方式**：取消、Esc、点遮罩、弹窗开着时切换路由，都不能发出登记请求；连点两下只出现一个弹窗。（Task 8）

---

### Task 1: 设计 token 与组件类

**Files:**
- Modify: `web/src/styles/app.css`

**Interfaces:**
- Produces: `@theme` token `--text-hero`、`--text-hero-lg`、`--text-display`、`--text-stat`、`--text-title`、`--shadow-overlay`、`--shadow-hero`、`--animate-rise`、`--animate-float-up`、`--animate-bump`、`--animate-wiggle`；组件类 `.surface`、`.inset`、`.divided`、`.stagger`、`.field-label`、`.status-chip`、`.float-chip`、`.page-enter-active`、`.btn-soft.btn-primary`。完整代码见设计文档附录 D。

- [ ] **Step 1: 起服务、写检查、跑出红**（按"固定流程"；检查代码如下）

```python
vp(1280, 820); login('parent'); go('/parent/overview', 'dark'); shot('t1-overview-d-dark')
dark_hierarchy('.card')
check('theme-color meta = page colour', js("document.querySelector('meta[name=theme-color]').getAttribute('content')") == '#1b1917')
logout(); vp(390, 844, 2, True); go('/login', 'light'); shot('t1-login-m-light'); go('/login', 'dark'); shot('t1-login-m-dark')
dark_hierarchy('.card')
names = js("(() => { const t = new Set(); for (const s of document.styleSheets) { try { for (const r of s.cssRules) { for (const k of ['.surface', '.divided', '.stagger', '.field-label', '.status-chip', '.float-chip', '.page-enter-active']) if (r.cssText.includes(k)) t.add(k); } } catch (e) {} } return [...t].sort().join(' '); })()")
check('compiled css has the component classes', names == '.divided .field-label .float-chip .page-enter-active .stagger .status-chip .surface', names)
no_horizontal_scroll(360)
```

  预期红：`dark theme: page darker than card` 与 `compiled css has the component classes` 失败。（字号、阴影这些工具类此刻还没有任何页面使用，Tailwind 不会生成，所以这里只核对组件类；工具类在后续任务用到时自然出现。）

- [ ] **Step 2: 实现**（设计文档 §4、§5.1、附录 D）

  深色主题的 `--color-base-100` 改为 `#232019`、`--color-base-200` 改为 `#1b1917`（浅色主题不动）；在 `@theme` 里加入字号、阴影、动效 token 与关键帧；在 `@layer components` 里加入全部组件类；`.toast-fade-enter-from` / `.toast-fade-leave-to` 加 `transform: translateY(8px)`，`.toast-fade-enter-active` / `.toast-fade-leave-active` 的 `transition` 加 `transform 0.2s`。保留现有的 `.btn { --fontsize: 1rem }`、`--color-brand-ink`、`--animate-pop`、减少动态效果 kill-switch。更新文件头注释，写明刻度。

- [ ] **Step 3: 静态核对**

  Run: `grep -nE 'color-base-(100|200): #' web/src/styles/app.css`
  Expected: 4 行；其中 `--color-base-100: #232019` 与 `--color-base-200: #1b1917` 各 1 行。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t1-overview-d-dark`、`t1-login-m-light`、`t1-login-m-dark`）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（1/9）— 设计 token、表面三层与深色主题层级`

### Task 2: 布局——侧栏分组与居中、页签、路由过渡

**Files:**
- Modify: `web/src/layouts/KidLayout.vue`
- Modify: `web/src/layouts/ParentLayout.vue`
- Modify: `web/src/components/ui/ThemeToggle.vue`（只加一个可选 `compact` prop，默认行为不变）

**Interfaces:**
- Consumes: Task 1 的 `.page-enter-active`、`text-title`。
- Produces: 两个布局的 `<main>` 里都是 `<RouterView v-slot="{ Component }"><Transition name="page" mode="out-in" appear><component :is="Component" /></Transition></RouterView>`。家长端侧栏的结构固定为：`<aside>` → 品牌区 `<div>` → `<nav class="flex-1 overflow-y-auto" aria-label="家长端导航">`（里面依次是三个分组标签 `<div>` 与各自对应的 `<ul class="menu">`，共 7 个链接）→ 账户区 `<div>`；侧栏链接带 `min-h-11 content-center rounded-xl`。导航项的 `to` / `label` / `icon` 在 `ParentLayout.vue` 里只定义一次（分三组），手机页签用的扁平顺序不变。`ThemeToggle` 新增可选 prop `compact?: boolean`：为真时按钮 `btn btn-ghost btn-circle btn-sm`、图标 `size-4`；不传时与现在完全一样。孩子端底部 Tab 当前项的图标容器带 `rounded-full bg-primary/12`。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
vp(1280, 820); login('parent'); go('/parent/overview', 'light'); shot('t2-overview-d-light')
check('sidebar groups in order', texts('aside nav > div') == ['日常', '设置', '记录'], texts('aside nav > div'))
check('sidebar has 7 links', count('aside nav ul a') == 7, count('aside nav ul a'))
check('sidebar hrefs in nav order', js("[...document.querySelectorAll('aside nav ul a')].map(a => a.getAttribute('href'))") == ['/parent/overview', '/parent/earn', '/parent/children', '/parent/items', '/parent/prizes', '/parent/draws', '/parent/bills'])
a = rect('aside nav a[aria-current="page"]'); i = rect('aside nav a[aria-current="page"] svg')
check('active item vertically centred', bool(a and i) and abs((i[1] - a[1]) - (a[1] + a[3] - i[1] - i[3])) <= 1, (a, i))
check('active item >= 44 high', bool(a) and a[3] >= 44, a)
check('brand block', text('aside .text-title') == '好习惯' and text('aside') != '', text('aside .text-title'))
js("window.__seenPage = false; new MutationObserver(() => { if (document.querySelector('.page-enter-active')) window.__seenPage = true; }).observe(document.body, { subtree: true, childList: true, attributes: true }); true")
js("document.querySelector('aside nav a[href=\"/parent/earn\"]').click(); true")
wait_for("location.pathname === '/parent/earn'")
check('route transition applied on navigation', js("window.__seenPage") is True)
go('/parent/overview', 'dark'); shot('t2-overview-d-dark')
vp(390, 844, 2, True); go('/parent/overview', 'light'); shot('t2-overview-m-light')
check('mobile header has brand mark', count('header .bg-primary svg') == 1, count('header .bg-primary svg'))
min_touch('header button'); min_touch('.tabs .tab'); no_horizontal_scroll(360)
login('yu'); go('/kid/home', 'light'); shot('t2-kid-home-m-light')
check('bottom tab pill on active item', count('nav.fixed a[aria-current="page"] .rounded-full') == 1)
min_touch('nav.fixed a'); no_horizontal_scroll(360)
vp(1280, 820); go('/kid/home', 'light'); shot('t2-kid-home-d-light')
check('desktop tabs >= 44 high', bool(rect('.tabs .tab')) and rect('.tabs .tab')[3] >= 44, rect('.tabs .tab'))
```

  预期红：分组、居中、品牌区、路由过渡、页签胶囊等断言失败。

- [ ] **Step 2: 实现**（设计文档 §5.2）

  家长端侧栏按效果图重组：品牌区（星标 `size-9` + `rounded-xl`、"好习惯" `text-title`、"家长端" 说明）；三个分组标签；导航项按设计文档第 161 行分三组（日常 = 总览、登记得分；设置 = 孩子、积分项、奖品与定价；记录 = 待发放、账单），`to` / `label` / `icon` 只写一次，侧栏用嵌套 `v-for` 渲染（外层用 `<template v-for>` 包"分组标签 `<div>` + `<ul class="menu">`"，不引入额外包裹元素），手机页签的扁平顺序不变；链接类名按 Interfaces，当前项加 `bg-primary/10 font-semibold text-brand-ink`；底部账户区的主题按钮写 `<ThemeToggle compact />`，不在布局里内联主题按钮。手机顶栏加品牌标记。孩子端顶栏昵称 `text-lg font-semibold`、桌面页签当前项 `font-semibold`、`<main>` 间距、底部 Tab 按效果图。两个布局的 `<main>` 内边距按 §5.2。

- [ ] **Step 3: 静态核对**

  Run: `grep -c 'name="page"' web/src/layouts/KidLayout.vue web/src/layouts/ParentLayout.vue && grep -c 'content-center' web/src/layouts/ParentLayout.vue && grep -c "to: '/parent/" web/src/layouts/ParentLayout.vue`
  Expected: 两个布局各 1 处 `name="page"`；`content-center` 1 处（在 `v-for` 渲染侧栏链接的类名里）；`to: '/parent/` 7 处（每条路径只定义一次）。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t2-overview-d-light`、`t2-overview-d-dark`、`t2-overview-m-light`、`t2-kid-home-m-light`、`t2-kid-home-d-light`，并对照 `.superpowers/mockup-kit/pages/parent-overview.html` 与 `kid-home.html`）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（2/9）— 布局：侧栏分组与居中、页签、路由过渡`

### Task 3: 共享组件套用刻度

**Files:**
- Modify: `web/src/components/ui/PageHeader.vue`
- Modify: `web/src/components/ui/RowCard.vue`
- Modify: `web/src/components/ui/FormField.vue`
- Modify: `web/src/components/ui/EmptyState.vue`
- Modify: `web/src/components/ui/BaseDialog.vue`
- Modify: `web/src/components/ui/ToastHost.vue`

**Interfaces:**
- Consumes: Task 1 的 `.surface`、`.field-label`、`text-title`、`shadow-overlay`。
- Produces: `PageHeader` 的 h2 为 `h2.text-title`；`RowCard` 根节点为 `.surface`，内部 `px-5 py-4`；`EmptyState` 的圆形图标底为 `border border-base-300 bg-base-100 rounded-full`；`BaseDialog` 的 `.modal-box` 带 `border-0 shadow-overlay animate-pop`。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
vp(1280, 820); login('parent'); go('/parent/items', 'light'); shot('t3-items-d-light')
wait_for("document.querySelectorAll('main .surface').length >= 11")
check('page title 20px/600', style('h2.text-title', 'font-size') == '20px' and style('h2.text-title', 'font-weight') == '600')
check('rows are flat surfaces (11 rows)', count('main .surface') == 11 and style('main .surface', 'box-shadow') == 'none', (count('main .surface'), style('main .surface', 'box-shadow')))
check('row padding 20px / 16px', style('main .surface > div', 'padding-left') == '20px' and style('main .surface > div', 'padding-top') == '16px', (style('main .surface > div', 'padding-left'), style('main .surface > div', 'padding-top')))
vp(390, 844, 2, True); go('/parent/items', 'light'); shot('t3-items-m-light')
min_touch('main .surface button.btn'); no_horizontal_scroll(360)
vp(1280, 820); go('/parent/children', 'light'); shot('t3-children-d-light')
js("[...document.querySelectorAll('main .surface button.btn')].find(b => b.textContent.trim() === '停用').click(); true")
wait_for("!!document.querySelector('dialog[open]')")
check('confirm dialog: no border, overlay shadow', style('dialog[open] .modal-box', 'border-top-width') == '0px' and style('dialog[open] .modal-box', 'box-shadow') != 'none', (style('dialog[open] .modal-box', 'border-top-width'), style('dialog[open] .modal-box', 'box-shadow')))
check('confirm dialog copy unchanged', text('dialog[open] h3') == '停用 小明', text('dialog[open] h3'))
shot('t3-confirm-d-light')
js("document.querySelector('dialog[open] .modal-action .btn-ghost').click(); true"); wait_for("!document.querySelector('dialog[open]')")
check('cancel leaves child active', '停用' in texts('main .surface button.btn'))
vp(390, 844, 2, True); go('/parent/children', 'light')
js("[...document.querySelectorAll('main .surface button.btn')].find(b => b.textContent.trim() === '停用').click(); true"); wait_for("!!document.querySelector('dialog[open]')")
min_touch('dialog[open] .modal-action .btn'); shot('t3-confirm-m-light')
js("document.querySelector('dialog[open] .modal-action .btn-ghost').click(); true"); wait_for("!document.querySelector('dialog[open]')")
vp(1280, 820); go('/parent/draws', 'light')
js("window.__toastSeen = false; new MutationObserver(() => { if (document.querySelector('.toast .alert')) window.__toastSeen = true; }).observe(document.body, { subtree: true, childList: true }); true")
js("document.querySelector('main .surface button.btn-primary').click(); true")
check('toast appears after fulfil', wait_for('window.__toastSeen', timeout=5))
check('toast uses overlay shadow', style('.toast .alert', 'box-shadow') != 'none', style('.toast .alert', 'box-shadow'))
check('empty state circle is white with hairline', wait_for("document.querySelector('main .rounded-full') !== null", timeout=5) and style('main .rounded-full', 'border-top-width') == '1px' and style('main .rounded-full', 'background-color') == 'rgb(255, 255, 255)', (style('main .rounded-full', 'border-top-width'), style('main .rounded-full', 'background-color')))
shot('t3-draws-empty-d-light')
```

  预期红：标题字号、行的阴影与内边距、弹窗描边、toast 阴影、空状态底色等断言失败。（表单卡片、行内按钮字号、h3 字重属于 Task 9，这里不检查。）

- [ ] **Step 2: 实现**（设计文档 §5.3）

  六个组件按 Interfaces 与 §5.3 改类名。`FormField` 的标签改用 `.field-label`。`ToastHost` 的 `shadow-lg` 改为 `shadow-overlay`。不改任何组件的 props、事件和逻辑。

- [ ] **Step 3: 静态核对**

  Run: `grep -rnE 'shadow-(sm|md|lg)' web/src/components/ui/ || echo none`
  Expected: 输出 `none`。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t3-items-d-light`、`t3-items-m-light`、`t3-children-d-light`、`t3-confirm-d-light`、`t3-confirm-m-light`、`t3-draws-empty-d-light`）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（3/9）— 共享组件套用刻度`

### Task 4: 孩子端首页——主数字卡、得分指南、待选横幅

**Files:**
- Create: `web/src/components/TickNumber.vue`
- Modify: `web/src/components/BalanceCard.vue`
- Modify: `web/src/components/GuideList.vue`
- Modify: `web/src/components/PendingBanner.vue`
- Modify: `web/src/views/kid/HomeView.vue`

**Interfaces:**
- Consumes: Task 1 的 token 与组件类；Task 2 的路由过渡（视图自身不再加进场动画类）。
- Produces: `TickNumber.vue`，`defineProps<{ value: number; enter?: boolean }>()`。DOM：根 `<span class="inline-block">`（透传 class），内含 `<span class="sr-only">{{ value }}</span>` 与 `<span aria-hidden="true">{{ shown }}</span>`。行为：
  - 挂载时：`enter` 为真、`value > 0`、且未开启减少动态效果 → `shown` **同步**设为 `Math.round(value * 0.7)`，再用 `requestAnimationFrame` 在 600ms 内按 `1 - (1 - t)^3` 滚到 `value`；否则 `shown` 直接是 `value`。
  - `value` 变化时：起点取上一个 `shown`；若 `enter` 为真且这是首个大于 0 的值，起点取 `Math.round(value * 0.7)`；减少动态效果时直接显示新值。起点等于终点时不动画。
  - 每次动画结束时在根节点播放一次 `animate-bump`（先移除类、读一次 `offsetWidth`、再加回）。卸载时取消未完成的 `requestAnimationFrame`。
  - `HomeView` 新增 `loaded` 标记；`BalanceCard` 新增 prop `loaded: boolean`，为假时渲染 `<span class="text-hero lg:text-hero-lg opacity-0" aria-hidden="true">0</span>` 占位。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
rid = hold_raf()
vp(390, 844, 2, True); login('yu'); go('/kid/home', 'light')
wait_for("document.querySelector('.text-hero [aria-hidden=\"true\"]') !== null")
start = text('.text-hero [aria-hidden="true"]')
check('hero starts at 70% of 25 (=18), never 0', start == '18', start)
time.sleep(0.8); step_raf()
check('hero ends at 25', text('.text-hero [aria-hidden="true"]') == '25', text('.text-hero [aria-hidden="true"]'))
remove_init_script(rid)
check('screen reader gets final value', text('.text-hero .sr-only') == '25', text('.text-hero .sr-only'))
check('hero 64px, proportional figures', style('.text-hero', 'font-size') == '64px' and style('.text-hero', 'font-variant-numeric') == 'normal', (style('.text-hero', 'font-size'), style('.text-hero', 'font-variant-numeric')))
check('unit next to hero', text('section[aria-label="我的积分"] .text-lg') == '分', text('section[aria-label="我的积分"] .text-lg'))
check('no chips inside hero card', count('section[aria-label="我的积分"] .rounded-full') == 0, count('section[aria-label="我的积分"] .rounded-full'))
shot('t4-kid-home-m-light'); go('/kid/home', 'dark'); shot('t4-kid-home-m-dark')
vp(1280, 820); go('/kid/home', 'light'); shot('t4-kid-home-d-light')
check('hero 72px on desktop', style('.text-hero', 'font-size') == '72px', style('.text-hero', 'font-size'))
vp(390, 844, 2, True); go('/kid/home', 'light')
wait_for("document.querySelectorAll('details ul > li').length === 11")
check('guide meta count', sq(text('details summary')).endswith('11项'), sq(text('details summary')))
check('guide first row', sq(text('details ul > li')) == '托管课写完作业+7分', sq(text('details ul > li')))
check('guide rows separated by hairlines', style('details ul > li', 'border-top-width') == '0px' and style('details ul > li:nth-child(2)', 'border-top-width') == '1px', (style('details ul > li', 'border-top-width'), style('details ul > li:nth-child(2)', 'border-top-width')))
check('guide rows have no pill background', style('details ul > li', 'background-color') == 'rgba(0, 0, 0, 0)', style('details ul > li', 'background-color'))
check('guide title 16px/600', style('details summary', 'font-size') == '16px' and style('details summary', 'font-weight') == '600')
check('divider inset matches text', abs(rect('details ul > li')[0] - rect('details')[0] - 1 - 20) <= 1, (rect('details ul > li')[0], rect('details')[0]))
check('banner: no infinite pulse, wiggle twice', count('.animate-pulse') == 0 and count('.animate-wiggle') == 1 and style('.animate-wiggle', 'animation-iteration-count') == '2', (count('.animate-pulse'), count('.animate-wiggle'), style('.animate-wiggle', 'animation-iteration-count')))
min_touch('nav.fixed a'); no_horizontal_scroll(360)
reduced_motion(True); go('/kid/home', 'light'); wait_for("document.querySelector('.text-hero') !== null")
check('reduced motion: final value at once', text('.text-hero [aria-hidden="true"]') == '25', text('.text-hero [aria-hidden="true"]'))
dur = style('.animate-wiggle', 'animation-duration'); dur_ms = float(dur[:-2]) if dur.endswith('ms') else float(dur.rstrip('s')) * 1000   # Chrome 把 computed 时长规范化为秒（0.01ms → '1e-05s'），按数值比较
check('reduced motion: css animations off', dur_ms <= 0.01, dur)
reduced_motion(False)
reduced_motion(True)   # 后台标签页里 rAF 不推进，数字会停在起点；下面几条读数值的断言在"减少动态效果"下读取（直接是最终值）
login_token(new_child('零分孩子')); go('/kid/home', 'light'); wait_for("document.querySelector('.text-hero') !== null")
check('zero balance shows 0', text('.text-hero [aria-hidden="true"]') == '0', text('.text-hero [aria-hidden="true"]'))
login_token(new_child('四位数孩子', adjust=1234)); go('/kid/home', 'light'); wait_for("document.querySelector('.text-hero') !== null")
check('four-digit balance', text('.text-hero [aria-hidden="true"]') == '1234', text('.text-hero [aria-hidden="true"]'))
hr = rect('.text-hero'); cr = rect('section[aria-label="我的积分"]')
check('four-digit hero stays inside card', bool(hr and cr) and hr[0] >= cr[0] and hr[0] + hr[2] <= cr[0] + cr[2], (hr, cr))
no_horizontal_scroll(360); shot('t4-kid-home-m-1234')
reduced_motion(False)
```

  说明：`hold_raf()` 在页面脚本运行前冻结 `requestAnimationFrame`，所以能确定地读到起点值；读完后 `step_raf()` 放行。`rect('details')[0]` 是卡片左缘，行左缘应比它大 1px 描边 + 20px 内边距。

  预期红：起点值、字号、指南分隔线、横幅动效、减少动态效果等断言失败（现状没有 `.text-hero`，第一处 `wait_for` 会超时——超时后的断言一律算失败，属正常）。

- [ ] **Step 2: 实现**（设计文档 §5.3 `TickNumber`、§5.4；对照 `.superpowers/mockup-kit/pages/kid-home.html`）

  `HomeView`：`loaded` 标记，`flex flex-col gap-6`。`BalanceCard`：按效果图重写模板并接收 `loaded`。`GuideList`：`details.surface.collapse.collapse-arrow`，清单 `ul.stagger.divided.px-5.lg:px-6`。`PendingBanner`：去掉 `animate-pulse`，`🎁` 放进 `inline-block animate-wiggle text-2xl`。文案一律不改。

- [ ] **Step 3: 静态核对**

  Run: `grep -c 'animate-pulse' web/src/components/PendingBanner.vue web/src/views/kid/HomeView.vue web/src/components/BalanceCard.vue || echo none`
  Expected: 输出 `none`。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t4-kid-home-m-light`、`t4-kid-home-m-dark`、`t4-kid-home-d-light`、`t4-kid-home-m-1234`，与效果图 `kid-home.html` 逐项对照）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（4/9）— 孩子端首页：主数字卡、得分指南、待选横幅`

### Task 5: 账单行

**Files:**
- Modify: `web/src/components/BillRow.vue`
- Modify: `web/src/views/kid/BillsView.vue`
- Modify: `web/src/views/parent/BillsView.vue`

**Interfaces:**
- Consumes: Task 1 的 `.surface`、`.stagger`。
- Produces: `BillRow` 根节点 `.surface`；两个账单页的行容器都带 `stagger` 类。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
vp(390, 844, 2, True); login('ming'); go('/kid/bills', 'light'); shot('t5-kid-bills-m-light')   # 小明有 8 笔得分，下面的 :nth-child(8) 才存在（小雨只有 6 笔）
wait_for("document.querySelectorAll('.stagger .surface').length >= 4")
check('rows have no shadow', style('.stagger .surface', 'box-shadow') == 'none', style('.stagger .surface', 'box-shadow'))
check('row title 16px/500', style('.stagger .surface p', 'font-size') == '16px' and style('.stagger .surface p', 'font-weight') == '500', (style('.stagger .surface p', 'font-size'), style('.stagger .surface p', 'font-weight')))
d = js("(() => { const e = document.querySelector('.stagger .surface .tabular-nums'); const c = getComputedStyle(e); return [c.fontSize, c.fontWeight, c.fontVariantNumeric, e.textContent.trim()]; })()")
check('delta 18px/600, tabular', d[:3] == ['18px', '600', 'tabular-nums'], d)
check('row keeps balance snapshot', any('当时' in t for t in texts('.stagger .surface')), texts('.stagger .surface')[:2])
check('stagger delays 30ms apart', style('.stagger > *:nth-child(2)', 'animation-delay') == '0.03s' and style('.stagger > *:nth-child(8)', 'animation-delay') == '0.21s', (style('.stagger > *:nth-child(2)', 'animation-delay'), style('.stagger > *:nth-child(8)', 'animation-delay')))
no_horizontal_scroll(360)
vp(1280, 820); login('parent'); go('/parent/bills', 'light'); shot('t5-parent-bills-d-light')
wait_for("document.querySelectorAll('.stagger .surface').length >= 4")
check('parent bills use stagger', count('.stagger .surface') >= 4, count('.stagger .surface'))
go('/parent/bills', 'dark'); shot('t5-parent-bills-d-dark')
```

  预期红：阴影、字号字重、`stagger` 相关断言失败。

- [ ] **Step 2: 实现**（设计文档 §5.4 账单部分）

  `BillRow`：容器 `.surface`，内部 `flex items-center gap-3 px-5 py-3.5`；标题 `text-base font-medium`；增减 `text-lg font-semibold tabular-nums`，颜色映射不变；其余不变。两个账单页：行容器 `flex flex-col gap-2 stagger`。

- [ ] **Step 3: 重建、跑出绿、看截图**（`t5-kid-bills-m-light`、`t5-parent-bills-d-light`、`t5-parent-bills-d-dark`）

- [ ] **Step 4: 收尾并提交** — `feat(web): 视觉工艺升级（5/9）— 账单行`

### Task 6: 转盘页余额与弹窗

**Files:**
- Modify: `web/src/views/kid/WheelView.vue`
- Modify: `web/src/components/ChoiceModal.vue`
- Modify: `web/src/components/ResultModal.vue`

**Interfaces:**
- Consumes: Task 4 的 `TickNumber`；Task 1 的 `.surface`、`.float-chip`。
- Produces: 余额行容器带 `relative` 类，内含 `.text-stat` 的 `TickNumber`、单位与（扣分后 750ms 内）一个 `.float-chip`；`WheelView` 新增 `loaded` 标记。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
init_stub_raf()   # 在页面脚本运行前把 requestAnimationFrame 换成定时器：后台标签页里数字滚动和转盘动画才能推进
fid = delay_api('/api/wheel', 1500)
vp(390, 844, 2, True); login('ming'); go('/kid/wheel', 'light'); time.sleep(0.3)
check('balance hidden until loaded', count('.text-stat') == 0, count('.text-stat'))
remove_init_script(fid)
check('balance 47', wait_for("(() => { const e = document.querySelector('.text-stat [aria-hidden=\"true\"]'); return e && e.textContent === '47'; })()", timeout=8))
check('balance 28px in ink colour', style('.text-stat', 'font-size') == '28px' and style('.text-stat', 'color') == style('body', 'color'), (style('.text-stat', 'font-size'), style('.text-stat', 'color'), style('body', 'color')))
shot('t6-wheel-m-light')
hook_fetch('w')
js("document.querySelectorAll('main button.btn')[0].click(); true")
time.sleep(0.15)
check('float chip shows −5 at once', count('.float-chip') >= 1 and text('.float-chip') == '\u22125', (count('.float-chip'), text('.float-chip')))
check('balance rolls down to 42', wait_for("(() => { const e = document.querySelector('.text-stat [aria-hidden=\"true\"]'); return e && e.textContent === '42'; })()", timeout=8))
check('exactly one draw request', sum('/api/draws' in u for u in calls('w')) == 1, calls('w'))
check('result modal opens after spin', wait_for("!!document.querySelector('dialog[open]')", timeout=25))
check('result title', '恭喜抽中' in text('dialog[open]'), text('dialog[open]'))
shot('t6-wheel-result-m-light')
js("document.querySelector('dialog[open] button.btn').click(); true"); wait_for("!document.querySelector('dialog[open]')")
hook_fetch('w2'); js("document.querySelectorAll('main button.btn')[1].click(); true")
check('first-draw card appears', wait_for("document.body.textContent.includes('第一抽：')", timeout=25))
check('first-draw card is a flat surface', style('main .border-warning', 'box-shadow') == 'none', style('main .border-warning', 'box-shadow'))
shot('t6-wheel-first-m-light')
js("[...document.querySelectorAll('main button.btn')].find(b => b.textContent.includes('再转一次')).click(); true")
check('choice modal opens', wait_for("!!document.querySelector('dialog[open]')", timeout=25))
check('choice modal cannot be dismissed', count('dialog[open] .modal-backdrop') == 0, count('dialog[open] .modal-backdrop'))
shot('t6-wheel-choice-m-light')
js("document.querySelector('dialog[open] button.btn').click(); true")
check('result modal after choosing', wait_for("document.body.textContent.includes('恭喜抽中')", timeout=8))
js("document.querySelector('dialog[open] button.btn').click(); true"); wait_for("!document.querySelector('dialog[open]')")
check('balance 35 after double draw (7 points)', wait_for("(() => { const e = document.querySelector('.text-stat [aria-hidden=\"true\"]'); return e && e.textContent === '35'; })()", timeout=8))
no_horizontal_scroll(360)
```

  说明：`init_stub_raf()` 在页面脚本运行之前把 `requestAnimationFrame` 换成定时器（后台标签页里真实的 rAF 不推进），所以余额的进场滚动、扣分滚动和转盘的 4 秒动画都能按时间推进。定时器在后台会被节流到约 1 秒一次，因此读数值一律用 `wait_for` 轮询，不用固定 `sleep`。

  预期红：`.text-stat` 不存在、`.float-chip` 不出现、"第一抽"卡片带阴影。

- [ ] **Step 2: 实现**（设计文档 §5.4 转盘部分）

  余额行：`<p class="relative flex items-baseline justify-center gap-1.5">`，"当前积分" `text-base text-base-content/60`，数字 `<TickNumber :value="balance" enter class="text-stat text-base-content" />`（首次 `load()` 完成后才渲染），单位 `text-sm text-base-content/55`。`balance.value = resp.entry.balanceAfter` 这一行保持在接口返回处，紧接着设 `floatText = '−' + cost`，750ms 后清空（减少动态效果时直接不显示）。"第一抽"卡片改为 `surface mt-4 animate-pop border-warning bg-warning/10`。两个弹窗只把 `font-extrabold` 换成 `font-bold`。按钮、画布、流程一律不动。

- [ ] **Step 3: 静态核对**

  Run: `grep -c 'font-extrabold' web/src/components/ChoiceModal.vue web/src/components/ResultModal.vue web/src/views/kid/WheelView.vue || echo none`
  Expected: 输出 `none`。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t6-wheel-m-light`、`t6-wheel-result-m-light`、`t6-wheel-first-m-light`、`t6-wheel-choice-m-light`）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（6/9）— 转盘页余额与弹窗`

### Task 7: 家长端总览

**Files:**
- Modify: `web/src/views/parent/OverviewView.vue`

**Interfaces:**
- Consumes: Task 1 的 `.surface`、`.status-chip`、`text-display`、`text-stat`。
- Produces: 模板里的 `section[aria-label="概况"]`（三个直接子元素）、`article.surface`；脚本里的 `unfulfilledTotal`、`pendingCount` 两个 `computed`。不新增接口调用。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
kids, _ = api('GET', '/api/children', None, SESS['parent'])
ming_id = [c for c in kids['children'] if c['displayName'] == '小明'][0]['id']
vp(1280, 820); login('parent'); go('/parent/overview', 'light')
wait_for("document.querySelectorAll('article.surface').length >= 2")
shot('t7-overview-d-light')
t = texts('section[aria-label="概况"] > *')
check('three stat tiles', len(t) == 3, t)
s = [x.replace(' ', '') for x in t]
check('tile values', len(s) == 3 and s[0] == '今日得分13笔' and s[1].startswith('待发放奖品1个') and s[2] == '待选奖品1局', s)
check('pending tile links to draws', js("document.querySelectorAll('section[aria-label=\"概况\"] > *')[1].getAttribute('href')") == '/parent/draws')
check('status dot coloured when > 0', style('section[aria-label="概况"] > *:nth-child(2) .rounded-full', 'background-color') not in ('rgba(0, 0, 0, 0)', ''), style('section[aria-label="概况"] > *:nth-child(2) .rounded-full', 'background-color'))
check('stat value 28px', style('section[aria-label="概况"] > *:first-child .text-stat', 'font-size') == '28px', style('section[aria-label="概况"] > *:first-child .text-stat', 'font-size'))
check('balance 32px, proportional', style('article.surface .text-display', 'font-size') == '32px' and style('article.surface .text-display', 'font-variant-numeric') == 'normal', (style('article.surface .text-display', 'font-size'), style('article.surface .text-display', 'font-variant-numeric')))
check('balance in ink colour', style('article.surface .text-display', 'color') == style('h3', 'color'), (style('article.surface .text-display', 'color'), style('h3', 'color')))
check('register link per child', js("document.querySelector('article.surface a.btn').getAttribute('href')") == '/parent/earn?child=' + str(ming_id), js("document.querySelector('article.surface a.btn').getAttribute('href')"))
check('status chips', texts('article.surface .status-chip') == ['1 个待发放', '待选奖品'], texts('article.surface .status-chip'))
check('cards have no shadow', style('article.surface', 'box-shadow') == 'none', style('article.surface', 'box-shadow'))
go('/parent/overview', 'dark'); shot('t7-overview-d-dark')
vp(390, 844, 2, True); go('/parent/overview', 'light'); shot('t7-overview-m-light')
min_touch('article.surface a.btn'); no_horizontal_scroll(360)
fid = delay_api('/api/overview', 1500); go('/parent/overview', 'light'); time.sleep(0.3)
check('nothing shown before data arrives (no 0 flash)', count('section[aria-label="概况"]') == 0 and count('article.surface') == 0 and '笔' not in text('main'), (count('section[aria-label="概况"]'), text('main')))
check('content appears after data', wait_for("document.querySelectorAll('article.surface').length >= 2", timeout=8))
remove_init_script(fid)
eid = fail_api('/api/overview', 500); go('/parent/overview', 'light'); time.sleep(0.8)
check('api error: title only, no crash', text('h2') == '总览' and count('section[aria-label="概况"]') == 0 and count('article.surface') == 0, (text('h2'), count('section[aria-label="概况"]')))
remove_init_script(eid)
new_child('名字特别长的一个孩子', adjust=1234)
go('/parent/overview', 'light'); wait_for("document.querySelectorAll('article.surface').length >= 3"); shot('t7-overview-m-long')
hs = js("[...document.querySelectorAll('article.surface')].map(e => Math.round(e.getBoundingClientRect().height))")
check('long name does not grow the card', len(hs) >= 3 and len(set(hs)) == 1, hs)
check('long name is truncated', '名字特别长' in texts('article.surface h4')[2] and '…' not in texts('article.surface h4')[2], texts('article.surface h4'))
no_horizontal_scroll(360)
```

  说明：`delay_api` / `fail_api` 用 `Page.addScriptToEvaluateOnNewDocument` 在页面脚本之前接管 `fetch`，所以必须在 `go()` 之前调用、用完必须 `remove_init_script`。

  预期红：三格统计、状态胶囊、字号、数据未返回时的表现、长名字卡片高度等断言失败。

- [ ] **Step 2: 实现**（设计文档 §5.5 总览部分；对照 `.superpowers/mockup-kit/pages/parent-overview.html`）

  新增 `unfulfilledTotal`、`pendingCount`。概况与孩子两节包在 `v-if="info"` 里。三格统计、状态点（大于 0 时 `bg-warning` / `bg-accent`，为 0 时 `bg-base-300`）、孩子卡片、状态胶囊、"登记得分"软按钮按效果图。孩子卡片里的名字是 `<h4 class="truncate text-base font-semibold">`（效果图如此；`h3` 是"孩子"分区标题）。无孩子时的 `EmptyState` 不变。

- [ ] **Step 3: 重建、跑出绿、看截图**（`t7-overview-d-light`、`t7-overview-d-dark`、`t7-overview-m-light`、`t7-overview-m-long`，与效果图逐项对照）

- [ ] **Step 4: 收尾并提交** — `feat(web): 视觉工艺升级（7/9）— 家长端总览`

### Task 8: 家长端登记得分——布局对调、白底积分项、登记前确认

**Files:**
- Modify: `web/src/views/parent/EarnView.vue`

**Interfaces:**
- Consumes: Task 4 的 `TickNumber`；Task 1 的 `.surface`、`.field-label`、`.float-chip`、`text-stat`；现有 `useDialogs().confirm({ title, message, confirmText })`。
- Produces: 选人胶囊 `button[aria-pressed]`；积分项 `.grid` 里每项是 `<span class="relative block">` 包一个 `button.btn`；脚本里 `confirming`（防重入）与 `float`（`{ id: number; text: string } | null`）两个 `ref`。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
vp(1280, 820); login('parent'); go('/parent/earn', 'light')
wait_for("document.querySelectorAll('.grid button.btn').length === 11")
shot('t8-earn-d-light')
c1 = card_rect('给谁登记'); c3 = card_rect('手动调整'); g = rect('.grid')
check('desktop: cards on the left, tiles on the right', bool(c1 and c3 and g) and c1[0] < g[0] and c3[0] < g[0] and c1[2] == 320 and c3[1] > c1[1] + c1[3], (c1, c3, g))
go('/parent/earn', 'dark'); shot('t8-earn-d-dark')
vp(390, 844, 2, True); go('/parent/earn', 'light'); shot('t8-earn-m-light')
c1 = card_rect('给谁登记'); c3 = card_rect('手动调整'); g = rect('.grid')
check('mobile order: picker, tiles, adjust', bool(c1 and c3 and g) and c1[1] < g[1] < c3[1], (c1, c3, g))
min_touch('button[aria-pressed]'); min_touch('.grid button.btn'); min_touch('main input'); min_touch('main .btn-outline'); no_horizontal_scroll(360)
vp(1280, 820); go('/parent/earn', 'light'); wait_for("document.querySelectorAll('.grid button.btn').length === 11")
sel = js("(() => { const b = document.querySelector('button[aria-pressed=\"true\"]'); const c = getComputedStyle(b); return [c.backgroundColor, c.color, b.getAttribute('aria-pressed')]; })()")
check('selected chip: tinted, brand-ink text, aria-pressed', sel[2] == 'true' and sel[1] == 'rgb(168, 74, 8)' and sel[0] != 'rgba(0, 0, 0, 0)', sel)
check('unselected chip has aria-pressed=false', js("document.querySelectorAll('button[aria-pressed=\"false\"]').length") == 1)
tile = js("(() => { const b = document.querySelector('.grid button.btn'); const n = b.querySelector('.text-stat'); const t = b.lastElementChild; return [getComputedStyle(b).backgroundColor, getComputedStyle(n).fontSize, getComputedStyle(t).fontSize, n.textContent.replace(/\\s+/g, '')]; })()")
check('tile: white, number 28px, title 14px', tile[0] == 'rgb(255, 255, 255)' and tile[1] == '28px' and tile[2] == '14px' and tile[3] == '+7分', tile)
js("(() => { const c = document.querySelector('main .btn-outline').closest('.surface'); const n = c.querySelector('input[type=number]'); const t = c.querySelector('input:not([type=number])'); n.value = '-5'; n.dispatchEvent(new Event('input', { bubbles: true })); t.value = '验证'; t.dispatchEvent(new Event('input', { bubbles: true })); return true; })()")   # 空字段时按钮必然禁用，而 daisyUI 把所有禁用按钮统一渲染成 10% 着色，看不出是否 outline——填上字段让它启用再量
check('adjust button is outlined (enabled)', wait_for("!document.querySelector('main .btn-outline').disabled") and style('main .btn-outline', 'background-color') == 'rgba(0, 0, 0, 0)', (style('main .btn-outline', 'background-color'), style('main .btn-outline', 'border-top-color')))
check('inputs have visible labels', texts('main label.field-label') == ['备注（可不填）', '分值', '原因'], texts('main label.field-label'))
hook_fetch('e')
js("document.querySelector('.grid button.btn').click(); true"); wait_for("!!document.querySelector('dialog[open]')")
dt = sq(text('dialog[open]'))
check('confirm copy', text('dialog[open] h3') == '登记得分' and '给小明记一笔「托管课写完作业」，+7分。' in dt and sq(text('dialog[open] .btn-primary')) == '登记' and sq(text('dialog[open] .btn-ghost')) == '取消', dt)
shot('t8-earn-confirm-d-light')
check('no request before confirm', not any('/api/earnings' in u for u in calls('e')), calls('e'))
js("document.querySelector('dialog[open] .modal-action .btn-ghost').click(); true"); wait_for("!document.querySelector('dialog[open]')")
check('cancel: no request', not any('/api/earnings' in u for u in calls('e')), calls('e'))
js("document.querySelector('.grid button.btn').click(); true"); wait_for("!!document.querySelector('dialog[open]')")
js("document.querySelector('dialog[open]').dispatchEvent(new Event('cancel', { cancelable: true })); true"); wait_for("!document.querySelector('dialog[open]')")
check('Esc: no request', not any('/api/earnings' in u for u in calls('e')), calls('e'))
js("document.querySelector('.grid button.btn').click(); true"); wait_for("!!document.querySelector('dialog[open]')")
js("document.querySelector('dialog[open] .modal-backdrop button').click(); true"); wait_for("!document.querySelector('dialog[open]')")
check('backdrop: no request', not any('/api/earnings' in u for u in calls('e')), calls('e'))
js("document.querySelector('.grid button.btn').click(); true"); wait_for("!!document.querySelector('dialog[open]')")
js("document.querySelector('aside nav a[href=\"/parent/overview\"]').click(); true"); wait_for("location.pathname === '/parent/overview'")
check('route change closes dialog, no request', count('dialog[open]') == 0 and not any('/api/earnings' in u for u in calls('e')), calls('e'))
go('/parent/earn', 'light'); wait_for("document.querySelectorAll('.grid button.btn').length === 11"); hook_fetch('e2'); stub_raf()   # 让确认后的数字滚动在后台标签页里也能推进
js("const t = document.querySelector('.grid button.btn'); t.click(); t.click(); true"); wait_for("!!document.querySelector('dialog[open]')")
check('double tap opens one dialog', count('dialog[open]') == 1, count('dialog[open]'))
js("document.querySelector('dialog[open] .modal-action .btn-primary').click(); true")
check('float +7 appears above the tile', wait_for("document.querySelector('.float-chip') !== null", timeout=3) and text('.float-chip') == '+7', text('.float-chip'))
shot('t8-earn-after-confirm-d-light', settle=0.0)
check('one request after confirm', sum('/api/earnings' in u for u in calls('e2')) == 1, calls('e2'))
check('toast text unchanged', '已记 +7 分' in text('.toast'), text('.toast'))
check('chip rolls to 54', wait_for("(() => { const e = document.querySelector('button[aria-pressed=\"true\"] [aria-hidden=\"true\"]'); return e && e.textContent === '54'; })()", timeout=8))
reduced_motion(True); go('/parent/earn', 'light'); wait_for("document.querySelectorAll('.grid button.btn').length === 11")
js("document.querySelector('.grid button.btn').click(); true"); wait_for("!!document.querySelector('dialog[open]')")
js("document.querySelector('dialog[open] .modal-action .btn-primary').click(); true"); time.sleep(0.4)
check('reduced motion: no float chip, value at once', count('.float-chip') == 0 and text('button[aria-pressed="true"] [aria-hidden="true"]') == '61', (count('.float-chip'), text('button[aria-pressed="true"] [aria-hidden="true"]')))   # 47 + 7（前面已登记一次）+ 7 = 61
reduced_motion(False)
new_child('名字特别长的一个孩子')
go('/parent/earn', 'light'); wait_for("document.querySelectorAll('.grid button.btn').length === 11")
check('long name: chips do not overflow', all((r[0] + r[2]) <= 1280 for r in js("[...document.querySelectorAll('button[aria-pressed]')].map(e => { const r = e.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map(Math.round); })")))
no_horizontal_scroll(360); vp(390, 844, 2, True); shot('t8-earn-m-long')
```

  预期红：布局位置、白底、确认框、浮动标签等断言全部失败（现状点积分项直接发请求：`no request before confirm` 会失败）。

- [ ] **Step 2: 实现**（设计文档 §5.5 登记得分部分；对照 `.superpowers/mockup-kit/pages/parent-earn.html` 与其中的 `#earn-confirm`）

  网格与三块的位置按 §5.5；DOM 顺序保持 给谁登记 → 积分项 → 手动调整。给谁登记卡片、选人胶囊（类名见 §5.5，选中态必须用工具类覆盖）、备注字段（标签 `备注（可不填）`）。积分项：外层 `<span class="relative block">`，按钮类名见 §5.5，数字 `text-stat text-brand-ink`。`earn(item)` 改为：先 `confirming` 防重入 → `await dialogs.confirm({...})` → `ok` 为假返回 → 原有请求与更新逻辑 → 成功后 `float = { id: item.id, text: '+' + item.points }`，750ms 后清空（减少动态效果时不设 `float`）。手动调整卡片：两个字段带 `.field-label` 标签，按钮 `btn btn-outline max-lg:btn-lg`，逻辑不变。卸载时清掉浮动标签的定时器。

- [ ] **Step 3: 静态核对**

  Run: `grep -c 'dialogs.confirm' web/src/views/parent/EarnView.vue && grep -c 'btn-soft btn-primary rounded-full border-primary/25' web/src/views/parent/EarnView.vue`
  Expected: 第一项 ≥ 1；第二项 0（选中态写法不是这个）。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t8-earn-d-light`、`t8-earn-d-dark`、`t8-earn-m-light`、`t8-earn-confirm-d-light`、`t8-earn-after-confirm-d-light`、`t8-earn-m-long`）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（8/9）— 登记得分：布局对调、白底积分项、登记前确认`

### Task 9: 只套刻度的页面与公开页

**Files:**
- Modify: `web/src/views/parent/ChildrenView.vue`
- Modify: `web/src/views/parent/ItemsView.vue`
- Modify: `web/src/views/parent/PrizesView.vue`
- Modify: `web/src/views/parent/DrawsView.vue`
- Modify: `web/src/views/LoginView.vue`
- Modify: `web/src/views/RegisterView.vue`

**Interfaces:**
- Consumes: Task 1 的 token、`.surface`；Task 3 的 `RowCard`、`FormField`。
- Produces: 无新接口。只改模板里的类名，脚本部分一行不动。

- [ ] **Step 1: 起服务、写检查、跑出红**

```python
vp(1280, 820); login('parent')
for path, name in [('/parent/children', 'children'), ('/parent/items', 'items'), ('/parent/prizes', 'prizes'), ('/parent/draws', 'draws')]:
    go(path, 'light'); wait_for("document.querySelector('main .surface') !== null"); shot('t9-%s-d-light' % name)
    check(name + ': cards have no shadow', js("[...document.querySelectorAll('main .surface')].every(e => getComputedStyle(e).boxShadow === 'none')"))
    check(name + ': section titles 600', js("[...document.querySelectorAll('main h3')].every(e => getComputedStyle(e).fontWeight === '600')"))
go('/parent/children', 'light')
check('row action buttons 14px on desktop', js("[...document.querySelectorAll('main .surface .btn-ghost')].length >= 3 && [...document.querySelectorAll('main .surface .btn-ghost')].every(e => getComputedStyle(e).fontSize === '14px')"))   # 只量行内操作按钮（规范定义为 btn-ghost）；表单主按钮保持 daisyUI 默认 16px，与总览 / 登记页一致
vp(390, 844, 2, True)
for path, name, extra in [('/parent/children', 'children', 'main .surface .btn-primary'), ('/parent/items', 'items', 'main .join .btn'), ('/parent/prizes', 'prizes', 'main .join .btn')]:
    go(path, 'light')
    check(name + ': touch targets', min_touch('main .surface button.btn') > 0 and min_touch(extra) > 0 and min_touch('main input') > 0)
    no_horizontal_scroll(360)
go('/parent/draws', 'light'); shot('t9-draws-m-light'); min_touch('main .surface button.btn-primary')
logout()
for theme in ('light', 'dark'):
    vp(390, 844, 2, True); go('/login', theme); shot('t9-login-m-' + theme)
    check('login card floats with overlay shadow (%s)' % theme, style('.card', 'box-shadow') != 'none', style('.card', 'box-shadow'))
    go('/register', theme); shot('t9-register-m-' + theme)
    check('auth title weight 700 (%s)' % theme, style('h1', 'font-weight') == '700', style('h1', 'font-weight'))
    vp(1280, 820); go('/login', theme); shot('t9-login-d-' + theme); dark_hierarchy('.card')
no_horizontal_scroll(360)
```

  预期红：卡片阴影、按钮字号、公开页标题字重等断言失败。

- [ ] **Step 2: 实现**（设计文档 §5.5"只套刻度"与 §5.6）

  四个家长端页面的表单卡片改为 `.surface p-5`（内部 `flex flex-col gap-3`），h3 用 `text-base font-semibold`，行标题字重各降一级，行内操作按钮改为 `btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm`。`DrawsView` 的「标记已发放」按钮加 `max-lg:btn-lg`（设计文档 §5.5）。徽章、`join` 输入组、`select`、空状态、加载与错误提示一律不动。登录/注册页：标题 `text-2xl font-bold`，卡片 `shadow-md` → `shadow-overlay`。**不碰脚本部分。**

- [ ] **Step 3: 全局静态核对**

  Run: `grep -rnE 'font-extrabold|shadow-(sm|md|lg)\b' web/src || echo none`
  Expected: 输出 `none`。
  Run: `grep -rn 'lg:btn-sm' web/src || echo none`
  Expected: 输出 `none`。

- [ ] **Step 4: 重建、跑出绿、看截图**（`t9-children-d-light`、`t9-items-d-light`、`t9-prizes-d-light`、`t9-draws-d-light`、`t9-draws-m-light`、`t9-login-m-light`、`t9-login-m-dark`、`t9-register-m-light`、`t9-register-m-dark`、`t9-login-d-light`、`t9-login-d-dark`）

- [ ] **Step 5: 收尾并提交** — `feat(web): 视觉工艺升级（9/9）— 其余页面与公开页套用刻度`

### Task 10: 整体验收矩阵

**Files:**
- Verify only. 发现问题就修复，并在同一个提交里说明：`fix(web): 视觉工艺升级整体验收修复`。

**Interfaces:**
- Consumes: 全部前序任务。

- [ ] **Step 1: 起服务、造数据**（按"固定流程"，用 `build`）

- [ ] **Step 2: 跑全量检查**

```python
init_stub_raf()   # 页面脚本运行前把 requestAnimationFrame 换成定时器：主数字的进场滚动、转盘画布在被遮挡的窗口里才能推进；定时器被节流到约 1s 一跳，所以下面等 1.2s 再截图
def snap(path, name):
    for theme in ('light', 'dark'):
        vp(390, 844, 1, True); go(path, theme); time.sleep(1.2); shot('t10-%s-m-%s' % (name, theme), settle=0.2)
        vp(1280, 820, 1, False); go(path, theme); time.sleep(1.2); shot('t10-%s-d-%s' % (name, theme), settle=0.2)
login('yu')
for path, name in [('/kid/home', 'kid-home'), ('/kid/wheel', 'kid-wheel'), ('/kid/bills', 'kid-bills')]:
    snap(path, name)
login('parent')
for path, name in [('/parent/overview', 'p-overview'), ('/parent/earn', 'p-earn'), ('/parent/children', 'p-children'), ('/parent/items', 'p-items'), ('/parent/prizes', 'p-prizes'), ('/parent/draws', 'p-draws'), ('/parent/bills', 'p-bills')]:
    snap(path, name)
logout()
for path, name in [('/login', 'login'), ('/register', 'register')]:
    snap(path, name)
# 12 个路由在 360 宽下都没有横向滚动
login('parent')
for path in ['/parent/overview', '/parent/earn', '/parent/children', '/parent/items', '/parent/prizes', '/parent/draws', '/parent/bills']:
    go(path, 'light'); no_horizontal_scroll(360)
login('yu')
for path in ['/kid/home', '/kid/wheel', '/kid/bills']:
    go(path, 'light'); no_horizontal_scroll(360)
logout()
for path in ['/login', '/register']:
    go(path, 'light'); no_horizontal_scroll(360)
# 关键触控目标
login('parent'); go('/parent/earn', 'light')
min_touch('button[aria-pressed]'); min_touch('.grid button.btn'); min_touch('main input'); min_touch('main .btn-outline')
go('/parent/overview', 'light'); min_touch('article.surface a.btn')
vp(1280, 820); go('/parent/overview', 'light'); a = rect('aside nav a[aria-current="page"]'); i = rect('aside nav a[aria-current="page"] svg')
check('sidebar active item centred', bool(a and i) and abs((i[1] - a[1]) - (a[1] + a[3] - i[1] - i[3])) <= 1)
```

- [ ] **Step 3: 逐张目检**

  用 Read 工具查看 `t10-*` 的全部 48 张截图。每张确认：没有横向滚动条、没有文字被裁切或重叠、深色主题下页面比卡片暗、卡片没有多余阴影、层级与效果图一致。发现问题的页面记录路由、视口、主题和现象。

- [ ] **Step 4: 手测三条流程（用浏览器脚本驱动，同 Task 6 与 Task 8 的检查代码）**

  登记得分：点积分项 → 确认框 → 取消 / Esc / 遮罩 / 切路由都不发请求 → 点「登记」后余额滚动、飘出 +N、出现原有 toast。孩子端首页：主数字滚动到位，横幅摇两下后静止。转盘：抽奖后余额向下滚动并飘出 −N，结果弹窗正常。

- [ ] **Step 5: 构建与后端测试**

  Run: `cd web && npm run build`
  Expected: 退出码 0。
  Run: `C:/Users/Ryan/sdk/go1.27.4/bin/go.exe test ./...`
  Expected: 全部 `ok`，退出码 0。

- [ ] **Step 6: 收尾**（停服务、清理；如有修复则提交）
