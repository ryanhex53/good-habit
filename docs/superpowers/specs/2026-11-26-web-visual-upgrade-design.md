# 好习惯（good-habit）· 前端视觉工艺升级 · 设计文档

- 日期：2026-11-26
- 状态：**已批准**——方向由用户确认（效果图第 2 版，选 A）；builder check 已完成；2026-11-26 用户批准本文（含 §9 的全部裁量）
- 上游文档：`docs/superpowers/specs/2026-11-25-good-habit-design.md`（产品与架构，本文只覆盖前端视觉层）
- 范围：`web/`（Vue 3.5 + TypeScript + Tailwind CSS 4.3 + daisyUI 5.8），全部 12 个路由页面

## 0. 用户已拍板的决策

| 决策点 | 结论 |
|---|---|
| 范围 | 视觉升级 → **工艺升级、识别不变** |
| 保留 | 品牌橙 + 暖白配色、圆角扁平、emoji 插画、系统字体、明暗双主题、页面结构与导航顺序 |
| 登记页：选人 + 备注 | 保留在卡片里（不改成"分区标题直接放页面上"） |
| 登记页：积分项 | 白底卡片，沿用现状配色，与总览页一致 |
| 登记页：登记操作 | **点积分项先弹确认框**，确认后才登记 |
| 登记页：桌面布局 | **两张卡片（给谁登记、手动调整）在左，积分项在右**（与现状对调） |
| 桌面侧栏 | 选中项的图标与文字垂直居中 |
| 确认方式 | 先看效果图再实施；效果图第 2 版已通过 |

## 1. 意图

七阶段重做之后界面"不够精致"，根源不在配色或结构，而在工艺：

1. 没有字号和间距的刻度，层级靠"到处加粗"来表达，结果是哪里都不突出。
2. 所有容器都是同一种"描边 + 阴影"卡片，页面读起来是一摞等重的盒子。
3. 积分是这个产品的主角，却没有被当成主角处理：五处余额各写各的，字号、字重、是否等宽都不统一。
4. 除了弹窗弹入，几乎没有反馈动效。

本次目标：在不动识别、不动信息架构的前提下，补上刻度、层次和反馈，让界面更有层级、更一致，并让"积分"成为一个会被记住的细节。

## 2. 目标 / 非目标 / 反目标

**目标**

1. 建立可复用的刻度：字号角色、间距、表面三层、两层阴影、一份克制的动效词汇表。
2. 覆盖全部 12 个路由页面（2 个公开页、3 个孩子端页、7 个家长端页）。
3. 积分数字统一处理，并且"会动"：任何地方积分变化，数字滚到新值并飘出 ±N。
4. 落实用户点名的登记页改动（卡片位置、白底、确认框）与侧栏居中。
5. 保住终审修复轮已经取得的保证：<lg 触控目标 ≥44px、小字号橙字用 `brand-ink`、明暗双主题可读、移动端页签不溢出、`prefers-reduced-motion` 下无动画。

**非目标（本轮不做）**

- 不换配色、字体、插画；不引入任何新依赖、字体文件或图片资源。
- 不改信息架构与导航顺序；**未出效果图的页面只套用刻度，不改布局与结构。**
- 不改文案（登记页确认框文案、备注标签的写法除外）。
- 不做加载 / 空 / 错误状态改造，不做客户端校验，不给「手动调整」的「调整」按钮加确认（用户未要求）。
- 不改转盘画布（`WheelCanvas.vue`：色板、扇区文字方向、动画曲线）。
- 不改 `EarnView` 现有的 2.2 秒防抖 toast 机制（`flash()`）。
- 不引入前端测试框架（上游 §2 非目标）。

**反目标（做了就算失败）**

- 任何页面在 360px 宽度下出现横向滚动条。
- 任何 <lg 触控目标高度或宽度小于 44px（本轮新写或改写的控件）。
- 深色主题下页面比卡片亮（层级倒挂），或出现不可读文字。
- 系统开启「减少动态效果」后仍有动画。
- `cd web && npm run build`（`vue-tsc -b && vite build`）失败，或 `go test ./...` 失败。
- 破坏用户本机开发环境：占用或结束 8080 端口上的 nginx、改动仓库根目录的 `good-habit.db`、在 vite dev server 运行时执行 `npm ci`。

## 3. 约束

- **技术栈**：Vue 3.5 `<script setup lang="ts">`、vue-router 4、Tailwind CSS 4.3、daisyUI 5.8；无 Pinia；无 `<style scoped>`；全局样式只有 `web/src/styles/app.css`；状态用模块级单例 composable。
- **字体**：系统字体栈不变。中文系统字体没有 800 字重，`font-extrabold` 实际是伪粗体 → 全项目移除 `font-extrabold`。
- **daisyUI 5.8.2 的既定行为**（改样式前先看 `web/node_modules/daisyui/components/*.css`，文件是压缩过的单行，用 grep 取规则，不要整文件读取）：
  - 嵌套层的优先级是父层 > 子层：`daisyui.l1`（hover / active / focus）> `daisyui.l1.l2`（颜色变体、`[aria-pressed=true]`）> `daisyui.l1.l2.l3`（基础与 `btn-soft` / `btn-ghost` / `btn-outline`）。带 `aria-pressed="true"` 的 `.btn` 会被强制成实心"按下"外观；要保留柔和的选中态，必须用工具类覆盖 `bg-*` / `border-*` / `text-*`（工具类在所有 daisyUI 层之上）。
  - `btn-soft btn-primary` 的文字色是 `--color-primary`（浅色 3.4:1）→ 必须用 `text-brand-ink` 或把 `.btn-soft.btn-primary` 的 `--btn-rest-fg` 设为 `--color-brand-ink`。
  - `.btn` 自带 `overflow: hidden` → 飘出的 "+N" 标签不能放在按钮里面，要放在包住按钮的定位容器里。
  - `.menu` 的行是 `display: grid; align-content: flex-start` → 加了 `min-h-11` 后内容贴上沿，必须加 `content-center`。
  - `btn-sm` 是 12px 字 / 32px 高；`btn-lg` 是 18px 字 / 48px 高。项目的 `.btn { --fontsize: 1rem }` 会被尺寸变体覆盖。
  - `.collapse-content` 展开时自带 `padding-bottom: 1rem`；`.collapse-title` 右侧留了 3rem 给箭头。
- **构建**：`cd web && npm run build`，产物在 `internal/webui/dist`（不入库，由 Go `embed`）。
- **验证方式**：没有自动化 UI 测试。每个任务的验证 = `npm run build` 通过 + 浏览器脚本化核对（见 §8）。
- **环境**（Ryan 的 Windows 开发机，见附录 C）。

## 4. 设计系统

### 4.1 字号刻度：每一档只担任一个角色

| Token | 字号 / 行高 / 字重 / 字距 | 用途 |
|---|---|---|
| `text-hero` | 64px / 1 / 700 / −0.02em；`lg:text-hero-lg` 为 72px | 孩子端首页主数字，每个视图至多一个 |
| `text-display` | 32px / 1.2 / 600 / −0.01em | 总览页孩子卡片的余额 |
| `text-stat` | 28px / 1.25 / 600 | 总览页统计数值；登记页积分项的数字；转盘页余额 |
| `text-title` | 20px / 1.4 / 600 | 页面标题（`PageHeader` 的 h2）；侧栏品牌名 |
| `text-base` | 16px | 正文；分区标题（h3，600）；清单行标题（500） |
| `text-sm` | 14px | 次要文字；表单标签（500）；侧栏导航项 |
| `text-xs` | 12px | 说明、时间、页签标签、侧栏分组标签、单位"分"（主数字旁的单位为 `text-lg`） |

- 字重只用 400 / 500 / 600 / 700。700 只用于主数字和弹窗标题（现有 `ConfirmDialog` / `PromptDialog` 的 `text-lg font-bold` 保持不变）。其余 `font-extrabold` 全部替换为 `font-bold` 或 `font-semibold`（见 §5）。
- **数字**：大号的独立数字（hero / display / stat）用比例数字，不加 `tabular-nums`；需要纵向对齐的数字（账单的增减、清单里的分值、选人胶囊里的余额）加 `tabular-nums`。
- **数字的颜色**：积分数字一律用文字色 `text-base-content`，不用品牌色给数字着色。品牌色只出现在：可交互元素（按钮、链接、选中态）、孩子端主数字卡、以及"动作性"的 "+N 分"（用 `text-brand-ink`，即登记页积分项、得分指南、积分项列表）。

### 4.2 颜色

色值全部不变。唯一调整：**深色主题里 `--color-base-100` 与 `--color-base-200` 的值对调**——纸面 `#232019`，页面 `#1b1917`，让页面比卡片暗一级，与浅色主题的层级方向一致（现状是反的）。`index.html` 与 `useTheme.ts` 里的 `theme-color`（`#1b1917` / `#f6f2ec`）仍是页面底色，不用改。

### 4.3 表面三层

| 层 | 写法 | 用在哪 |
|---|---|---|
| 页面 | `bg-base-200` | 布局根节点 |
| 纸面 `.surface` | `rounded-2xl border border-base-300 bg-base-100`，**无阴影** | 所有卡片与列表容器 |
| 内嵌 | `rounded-2xl bg-base-200`，无描边 | 只能放在纸面**里面**（直接放在页面上与页面同色，看不见）。本轮只用于状态胶囊 `.status-chip` |

阴影只保留两层：`shadow-overlay`（弹窗、toast、登录/注册卡片）与 `shadow-hero`（孩子端主数字卡）。现状里的 `shadow-sm` / `shadow-md` / `shadow-lg` 全部移除，改用描边。

### 4.4 间距（4px 网格）

- 页面留边：孩子端 `px-4 pt-6`，≥lg 为 `px-6 pt-8`；家长端 `p-4`，≥lg 为 `px-10 py-8`。
- 页面标题到内容：20px（`mt-5` 或 `PageHeader` 的 `mb-5`）。
- 区块之间：24px（`gap-6` / `mt-6`）；总览页"概况"到"孩子"一节为 `mt-8`。
- 卡片内边距：20px（`p-5`）。清单行水平内边距 20px，行高 ≥48px（`py-3`）。
- 网格间距：12px（`gap-3`）；控件之间 8px / 12px。
- 标签到控件：6px（`mb-1.5`）。

### 4.5 动效词汇表

| 名称 | 定义 | 用在哪 |
|---|---|---|
| **rise** | opacity 0→1、translateY 6px→0，220ms ease-out | 页面内容进场（路由切换与首次进入）；清单逐行错开 30ms，最多错开 8 行 |
| **press** | 按下时 `scale(0.97)` | 登记页积分项 |
| **tick** | 数字用 600ms ease-out 滚到新值，同时 `scale` 1→1.06→1（320ms） | 所有积分数字的变化；孩子端首页与转盘页首次显示时从当前值的 70% 滚到当前值 |
| **float** | "+N / −N" 胶囊上浮 28px 并淡出，700ms | 登记成功（在该积分项上方）；抽奖扣分（在余额旁） |
| **pop** | 现有 `animate-pop` | 弹窗 |
| **wiggle** | 左右各 8° 摇两下，500ms × 2 | 待选奖品横幅的 🎁，进页面时一次 |
| **toast** | 上移 8px + 淡入淡出，200ms | toast |

移除：待选横幅的 `animate-pulse` 无限闪烁。

无障碍：`prefers-reduced-motion: reduce` 时以上全部关闭。现有全局 kill-switch 已覆盖 CSS 动画与过渡；由 JS 驱动的 tick 与 float 必须读取 `matchMedia('(prefers-reduced-motion: reduce)')`，为真时直接显示最终值、不出浮动标签。数字滚动期间，读屏软件读到的必须是最终值（滚动的数字 `aria-hidden`，另放一份 `sr-only` 的最终值）。

## 5. 逐页规格

标记说明：**【已出效果图】** 的页面以效果图为准（附录 A）；其余页面为**只套刻度**：表面、字号、字重、阴影、间距按 §4 改，布局、结构、交互一律不动。标"裁量"的是建造者可以直接采用、也可以用等价方式实现的点。

### 5.1 全局样式 `web/src/styles/app.css`

1. 深色主题的 `--color-base-100` / `--color-base-200` 对调（§4.2）。
2. `@theme` 新增（完整代码见附录 D.1）：字号 token `--text-hero`、`--text-hero-lg`、`--text-display`、`--text-stat`、`--text-title`（含各自的 `--line-height` / `--font-weight` / `--letter-spacing`）；阴影 `--shadow-overlay`、`--shadow-hero`；动效 `--animate-rise`、`--animate-float-up`、`--animate-bump`、`--animate-wiggle` 及其 `@keyframes`（`--animate-pop` 保留）。
3. `@layer components` 新增：`.surface`、`.inset`、`.divided > * + *`（分隔线 `border-t border-base-300/80`）、`.stagger`（子元素依次 `animate-rise`，延迟 0/30/…/210ms，第 8 个起都是 210ms）、`.field-label`、`.status-chip`、`.float-chip`（浮动标签，见 §5.4）、路由过渡 `.page-enter-active { animation: rise 0.22s ease-out both }`；`.btn-soft.btn-primary { --btn-rest-fg: var(--color-brand-ink) }`。
4. `.toast-fade-*` 过渡加上 `transform: translateY(8px)` → `translateY(0)`。
5. 现有的 `.btn { --fontsize: 1rem }`、减少动态效果 kill-switch、`--color-brand-ink` 及其深色覆盖保持不变。

### 5.2 布局

**`layouts/KidLayout.vue`**【已出效果图】

- 顶栏：昵称 `text-lg font-semibold`（原 `font-bold`）。主题与登出按钮不变（44px 触控目标保持）。
- 桌面页签：当前项 `tab-active font-semibold text-brand-ink`。
- `<main>`：`mx-auto w-full max-w-2xl px-4 pt-6 pb-[calc(env(safe-area-inset-bottom)+5.5rem)] lg:px-6 lg:pt-8 lg:pb-16`。
- 手机底部 Tab 栏：每一项是图标容器（`flex h-7 w-12 items-center justify-center rounded-full`）+ 标签。当前项图标容器加 `bg-primary/12`，图标 `text-brand-ink`，标签 `text-xs font-semibold text-brand-ink`；非当前项整体 `text-base-content/60`，标签 `text-xs font-medium`。`min-h-14`、`aria-current`、安全区内边距保持。
- 路由过渡（裁量）：`<RouterView v-slot="{ Component }">` 包一层 `<Transition name="page" mode="out-in" appear>`，配合 5.1 的 `.page-enter-active`。两个布局都加。视图组件自身不再加进场动画类。

**`layouts/ParentLayout.vue`**【已出效果图】

- 手机顶栏：品牌标记（`inline-flex size-8 items-center justify-center rounded-lg bg-primary text-primary-content` + `BaseIcon name="star"` 的 `size-4`）+ "好习惯"（`text-lg font-semibold`）+ 用户名（`min-w-0 flex-1 truncate text-sm text-base-content/60`）+ 主题、登出按钮（不变）。
- 手机页签条：结构与滚动行为不变；当前项加 `font-semibold text-brand-ink`。
- 桌面侧栏：
  - 品牌区：标记 `size-9` + `rounded-xl` + `size-5` 星标；"好习惯" `text-title`；"家长端" `text-xs text-base-content/55`。
  - 导航分三组，分组标签 `px-3 pt-3 pb-1.5 text-xs font-medium text-base-content/50`（第一组上方 `pt-3`，其余 `pt-4`）：**日常** = 总览、登记得分；**设置** = 孩子、积分项、奖品与定价；**记录** = 待发放、账单。链接顺序与 `links` 数组不变。
  - 每个链接：`min-h-11 content-center rounded-xl`；当前项再加 `bg-primary/10 font-semibold text-brand-ink` 并保留 `aria-current`。
  - 底部：分隔线 + 用户名 + 主题、登出（`btn btn-ghost btn-circle btn-sm`，图标 `size-4`）。
- `<main>`：`mx-auto w-full max-w-5xl p-4 lg:px-10 lg:py-8`。

### 5.3 共享组件

- **`ui/PageHeader.vue`**：h2 用 `text-title`（去掉 `text-xl font-bold`），外层 `mb-5`，副标题不变。
- **`ui/RowCard.vue`**：容器由 `card … shadow-sm` 改为 `.surface`；内部 `flex-row flex-wrap items-center gap-3 px-5 py-4`；手机上 actions 独占一行的逻辑不变。
- **`ui/FormField.vue`**：标签改用 `.field-label`（去掉 daisyUI 的 `label` 类）；提示与错误的字号、颜色不变。
- **`ui/EmptyState.vue`**（裁量）：圆形图标底改为 `border border-base-300 bg-base-100`（现状是 `bg-base-200`，放在页面上看不见）。
- **`ui/BaseDialog.vue`**：`modal-box` 加 `border-0 shadow-overlay`，保留 `animate-pop`。
- **`ToastHost.vue`**：`alert` 的 `shadow-lg` 改为 `shadow-overlay`；过渡按 5.1。
- **`ConfirmDialog.vue` / `PromptDialog.vue`**：不改。
- **新增 `components/TickNumber.vue`**（裁量：也可以做成 composable）：
  - props：`value: number`、`enter?: boolean`。
  - 根节点是 `inline-block` 的 `<span>`，class 透传；内部一份 `sr-only` 的最终值 + 一份 `aria-hidden` 的滚动数字。
  - `value` 变化时从上一次显示值滚到新值（600ms，三次缓出 `1 - (1 - t)^3`），并播放一次 bump。
  - `enter` 为真时：**首次收到大于 0 的值**，从 `Math.round(value * 0.7)` 滚到当前值。挂载时值为 0（数据还没回来）不算"首次"，也不播放从 0 开始的滚动。
  - 减少动态效果时直接显示最终值。卸载时取消未完成的 `requestAnimationFrame`。

### 5.4 孩子端

**`views/kid/HomeView.vue` + `BalanceCard.vue` + `GuideList.vue` + `PendingBanner.vue`**【已出效果图】

- `HomeView`：`flex flex-col gap-6`（原 `gap-4`，去掉 `pt-2`）。
- `BalanceCard`：
  - 容器 `relative overflow-hidden rounded-2xl bg-linear-to-br from-accent to-primary text-primary-content shadow-hero`（去掉 `card` 与 `border-0`）；内层 `px-6 py-6 lg:px-8 lg:py-8`。
  - "我的积分"：`text-sm font-medium text-primary-content/80`（去掉胶囊）。
  - 数字行：`mt-2 flex items-baseline gap-2`；数字是 `<TickNumber :value="balance" enter class="text-hero lg:text-hero-lg" />`；单位 `text-lg font-medium text-primary-content/75`。
  - 数据返回前不显示数字：`HomeView` 用一个 `loaded` 标记，`BalanceCard` 在 `loaded` 为假时渲染一个同字号的透明占位（`text-hero lg:text-hero-lg opacity-0`，`aria-hidden="true"`）撑住高度，避免先闪一下 0。
  - 提示语原文不变：`mt-3 text-sm leading-relaxed text-primary-content/85`（去掉胶囊）。
- `PendingBanner`：去掉 `animate-pulse`；`🎁` 放进 `inline-block animate-wiggle text-2xl`；标题 `block text-base font-semibold`；选项行 `block truncate text-sm text-base-content/60`；"去选 ›" 为 `shrink-0 text-sm font-semibold text-brand-ink`。容器 `flex items-center gap-3 rounded-2xl border-2 border-dashed border-warning bg-warning/10 py-3 px-4`。
- `GuideList`：
  - `<details class="surface collapse collapse-arrow" open>`；`<summary class="collapse-title min-h-12 px-5 py-4 text-base font-semibold lg:px-6">`："🏅 得分指南" +（有积分项时）`<span class="ml-2 text-sm font-normal text-base-content/50">N 项</span>`。
  - 内容区 `collapse-content px-0 pt-0 pb-1`；`<ul class="stagger divided px-5 lg:px-6">`；每行 `flex items-center justify-between gap-3 py-3`；标题 `min-w-0 truncate text-base`；分值 `shrink-0 text-base font-semibold tabular-nums text-brand-ink`，"+N" 后跟 `<span class="ml-1 text-sm font-medium">分</span>`。
  - 无积分项时的提示文案不变。

**`views/kid/WheelView.vue`**（只套刻度 + 数字动效；画布、按钮布局、弹窗流程不变）

- 余额行：容器 `relative flex items-baseline justify-center gap-1.5`；"当前积分" `text-base text-base-content/60`；数字 `<TickNumber :value="balance" enter class="text-stat text-base-content" />`（原 `text-primary text-2xl font-bold tabular-nums`）；单位 `text-sm text-base-content/55`。
- 时机：`balance.value = resp.entry.balanceAfter` 这一行保持在接口返回处（现状位置不变）——数字在转盘开始转动的同时向下滚动，并在该行旁飘出 "−N"（`.float-chip`，放在这个定位容器里）。不等转盘转完：那时结果弹窗会盖住这一行。保险抽的扣分只在发起时发生一次，同样在此时触发。
- 余额行在首次 `load()` 完成后再渲染（`loaded` 标记），避免先显示 0。
- "第一抽"卡片：`surface mt-4 border-warning bg-warning/10`（去掉 `shadow-sm`，保留 `animate-pop`）。
- 两个抽奖按钮：布局与颜色不变。

**`ChoiceModal.vue` / `ResultModal.vue`**：`font-extrabold` → `font-bold`，其余不变。

**`views/kid/BillsView.vue` + `BillRow.vue`**（`BillRow` 同时服务家长端账单页）

- 列表容器：`flex flex-col gap-2 stagger`。
- `BillRow`：容器 `.surface`，去掉 `card` / `card-body` / `shadow-sm`；内部 `flex items-center gap-3 px-5 py-3.5`；标题 `text-base font-medium`（原 `text-sm font-semibold`）；时间 `text-xs text-base-content/60`；增减 `text-lg font-semibold tabular-nums`（原 `font-extrabold`），颜色映射不变；"当时 N" 不变。

### 5.5 家长端

**`views/parent/OverviewView.vue`**【已出效果图】

- 数据：新增两个派生值 `unfulfilledTotal`（各孩子 `unfulfilled` 之和）与 `pendingCount`（`hasPending` 的孩子数）。不新增接口调用。
- "概况"与"孩子"两节都包在 `v-if="info"` 里（数据未回来时只显示标题，不再闪一下三个 0）。
- 概况：`<section class="surface mt-5 grid grid-cols-3 divide-x divide-base-300/80" aria-label="概况">`，三格：
  1. 今日得分：数值 `text-2xl font-semibold sm:text-stat` + 单位 "笔"（`text-xs sm:text-sm text-base-content/55`）；标签 `text-xs sm:text-sm text-base-content/60`。
  2. 待发放奖品：整格是 `RouterLink` 到 `/parent/draws`，`px-4 py-4 transition-colors hover:bg-base-200/60 sm:px-5`；标签前有状态点 `size-1.5 rounded-full`（大于 0 为 `bg-warning`，为 0 为 `bg-base-300`）；数值 + "个"，右侧 `text-base-content/40` 的 "›"。
  3. 待选奖品：同上结构的状态点（大于 0 为 `bg-accent`）+ 数值 + "局"，不是链接。
  - 每一格内边距 `px-4 py-4 sm:px-5`。
- 孩子：标题 `<h3 class="mb-3 text-base font-semibold">孩子</h3>`；网格 `grid gap-4 sm:grid-cols-2 xl:grid-cols-3`。
- 孩子卡片 `<article class="surface p-5">`：
  - 首行 `flex items-center justify-between gap-3`：名字 `truncate text-base font-semibold`；状态胶囊 `.status-chip`（`有 unfulfilled > 0` 时显示，圆点 `bg-warning`，文字 "N 个待发放"；`hasPending` 时显示，圆点 `bg-accent`，文字 "待选奖品"；两者都有时并排，`flex flex-wrap gap-1.5`）。
  - 余额 `mt-4 flex items-baseline gap-1.5`：`text-display` + "分"（`text-sm text-base-content/55`）。
  - 操作 `mt-4`：`RouterLink` 到 `/parent/earn?child=${c.id}`，`btn btn-soft btn-primary rounded-full text-brand-ink hover:text-brand-ink max-lg:btn-lg`，文字 "登记得分"。
- 无孩子时的 `EmptyState` 不变。

**`views/parent/EarnView.vue`**【已出效果图】

- 标题 `PageHeader`；错误提示不变。
- 网格：`mt-5 flex flex-col gap-6 lg:grid lg:grid-cols-[320px_minmax(0,1fr)] lg:items-start lg:gap-x-8 lg:gap-y-6`。**DOM 顺序不变**（给谁登记 → 积分项 → 手动调整，手机上按此顺序堆叠）；桌面位置：给谁登记 `lg:col-start-1 lg:row-start-1`，积分项 `min-w-0 lg:col-start-2 lg:row-span-2 lg:row-start-1`，手动调整 `lg:col-start-1 lg:row-start-2`。
- 给谁登记卡片 `surface p-5`，内部 `flex flex-col gap-3`：h3 `text-base font-semibold`；选人胶囊；备注字段（标签 "备注（可不填）" 用 `.field-label`，占位符 "今天完成得很好"）。
- 选人胶囊（`v-for` 启用中的孩子）：
  - 选中：`btn btn-primary rounded-full border-primary/25 bg-primary/10 text-brand-ink hover:border-primary/25 hover:bg-primary/15 hover:text-brand-ink max-lg:btn-lg`，`aria-pressed="true"`。
  - 未选中：`btn btn-soft rounded-full max-lg:btn-lg`，`aria-pressed="false"`。
  - 内容：`<span class="font-medium">{{ c.displayName }}</span>` + `<TickNumber :value="c.balance" class="tabular-nums" />` + `<span class="text-xs">分</span>`。
- 积分项：`grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4`。每一项外面是 `<span class="relative block">`，里面是按钮：
  - `btn h-auto min-h-24 w-full flex-col justify-center gap-1 rounded-2xl border-base-300 bg-base-100 px-3 py-4 hover:border-primary/40 active:scale-[0.97] max-lg:btn-lg`，`:disabled="selected === null"`。
  - 数字 `text-stat text-brand-ink`："+N" + `<span class="ml-1 text-sm font-medium">分</span>`；标题 `w-full truncate text-sm font-normal text-base-content/75`。
  - 浮动标签：登记成功后，在刚点的这一项的 `<span class="relative block">` 里渲染 `<span class="float-chip" aria-hidden="true">+N</span>`，750ms 后移除。同一时刻只有一个。
- **确认流程**（行为变更）：点击积分项 →
  1. 没选孩子或 `toastVisible` 为真时直接返回（现状不变）。
  2. `const ok = await dialogs.confirm({ title: '登记得分', message: \`给 ${child.displayName} 记一笔「${item.title}」，+${item.points} 分。\`, confirmText: '登记' })`；`ok` 为假直接返回，不发请求。
  3. 之后的请求、本地余额更新、`flash()` 提示、清空备注、错误处理都与现状一致；成功后额外触发该积分项的浮动标签。
- 手动调整卡片 `surface p-5`：h3 `text-base font-semibold`；"分值"（`type="number"`，占位符 "扣分填负数，如 -20"）与 "原因"（占位符 "必填"）各带 `.field-label` 标签；按钮改为 `btn btn-outline max-lg:btn-lg`，禁用逻辑与提交逻辑不变。**不加确认框。**
- 空状态文案（"先去「孩子」页添加孩子"、"还没有积分项…"）不变。

**只套刻度的家长端页面**：`ChildrenView.vue`、`ItemsView.vue`、`PrizesView.vue`、`DrawsView.vue`、`parent/BillsView.vue`

- 表单卡片：`card border … shadow-sm` + `card-body` → `.surface p-5`，内部 `flex flex-col gap-3`；h3 `text-base font-semibold`。
- 列表里的 `RowCard` 按 5.3；行标题 `font-bold` / `font-semibold` → `font-semibold` / `font-medium`（各降一级）。
- 行内操作按钮：`btn btn-ghost max-lg:min-h-11 lg:btn-sm` → `btn btn-ghost max-lg:min-h-11 lg:h-8 lg:min-h-8 lg:px-2.5 lg:text-sm`（桌面 14px 字，不再用 12px 的 `btn-sm`）。
- `DrawsView.vue`：「标记已发放」按钮加 `max-lg:btn-lg`——现状在手机上只有 40px 高，不到 44px 的触控目标。
- 徽章（`badge badge-soft` 等）、`join` 输入组、`select`、空状态、加载与错误提示一律不变。
- `parent/BillsView` 的列表容器加 `stagger`。

### 5.6 公开页 `LoginView.vue` / `RegisterView.vue`（只套刻度）

- 标题 `text-2xl font-extrabold` → `text-2xl font-bold`。
- 卡片 `card … shadow-md` → `card … shadow-overlay`（保持"悬浮卡片"的观感；裁量：也可改成 `.surface`）。
- 其余不变。

## 6. 行为变更清单（全量）

1. **登记得分先弹确认框**（§5.5）。
2. **积分数字动效**：孩子端首页与转盘页余额首次显示时滚动；登记成功、抽奖扣分后数字滚动并飘出 ±N。
3. **路由切换与首次进入时内容上浮进场**。
4. **总览页数据未返回时不再显示三个 0**（只显示标题）。
5. 其余交互（防抖、校验、错误提示、弹窗行为、导航）均不变。

## 7. 留给建造者

- `TickNumber` 做成组件还是 composable；浮动标签的具体实现（v-if + 定时器，或 `<Transition>`）。
- 模板里直接写工具类，还是为重复出现的组合再抽组件类——以效果图页面的类名写法为准（附录 A），不要求逐字一致，但最终的 computed 样式必须一致。
- 登录/注册卡片用 `shadow-overlay` 还是 `.surface`。
- 任务的拆分与顺序。
- 文档中标注"裁量"的其余各点。

## 8. 验证

每个任务完成后必须给出以下证据，不能只说"已完成"：

1. `cd web && npm run build` 退出码为 0（含 `vue-tsc` 类型检查）。
2. 浏览器脚本化核对（用本机的 `browser-use` CLI，方法见附录 C）：对改动涉及的页面，在 390×844（DPR 2）与 1280×820 两个视口、浅色与深色两个主题下各截一张图，并**实际查看截图**。至少核对：
   - 没有 `font-extrabold` 残留：`grep -rn "font-extrabold" web/src` 无结果。
   - 360px 宽度下 `document.documentElement.scrollWidth <= 360`。
   - 本轮新写或改写的控件在 390px 宽度下 `getBoundingClientRect()` 的高与宽都 ≥ 44。
   - 深色主题下页面底色比卡片暗（`body` 背景的亮度低于 `.surface` 的亮度）。
   - 模拟 `prefers-reduced-motion: reduce` 时，数字直接显示最终值、不出现 `.float-chip`。
3. 全部任务完成后跑一遍整体验收矩阵：12 个路由 × 2 视口 × 2 主题，逐张目检；并手测以下流程：
   - 登记得分：点积分项 → 确认框出现且文案正确 → 取消 / Esc / 点遮罩都不发请求 → 点「登记」后余额滚动、飘出 +N、出现原有 toast。
   - 孩子端首页：主数字滚动到位；待选横幅的 🎁 摇两下后静止，不再持续闪烁。
   - 转盘：抽奖后余额向下滚动并飘出 −N。
   - 桌面侧栏：选中项图标上下间距相等。
   - 登记页桌面：两张卡片在左、积分项在右；手机上顺序为 选人 → 积分项 → 手动调整。
4. `go test ./...` 仍然通过（前端产物由 Go 嵌入，静态资源相关的测试不能坏）。

## 9. 建造者裁量记录

builder check 之后由作者定案、用户可在审阅时否决的各点：

1. 转盘页余额动效的时机：接口返回即更新，不等转盘转完（§5.4）。
2. 主数字进场：数据返回前不显示数字；数据到了从七成滚到当前值（§5.3、§5.4）。
3. 未出效果图的 9 个页面只套刻度，不改布局（§5）。
4. 总览页数据未返回时只显示标题，不再显示三个 0（§5.5）。
5. 登录 / 注册卡片保留悬浮感，用 `shadow-overlay`（§5.6）。
6. 空状态的圆形图标底改为白底描边（§5.3）。
7. 行内操作按钮在桌面端用 14px 字，不再用 12px 的 `btn-sm`（§5.5）。
8. 路由切换与首次进入时内容上浮进场（§5.2）。
9. 「手动调整」的「调整」按钮不加确认框——用户没有要求（§2）。
10. 待发放页的「标记已发放」按钮在手机上补到 48px 高（现状 40px，不到触控目标）——写实施计划时发现，一并修掉（§5.5）。

## 附录 A：效果图对照

效果图为一次性产物，不入库，位于本机 `D:\Ryan\Code\good-habit\.superpowers\mockup-kit\`：

- `mock.source.css`：本次 token 与组件类的完整源码（附录 D 的来源）。
- `pages/kid-home.html`、`pages/parent-overview.html`、`pages/parent-earn.html`：三页的参考标记（含确认框 `#earn-confirm`），响应式类名与 Vue 模板的写法一一对应。
- `pages/index.html`：给用户看的前后对比页。

如果这些文件已不存在，以本文 §4、§5 与附录 D 为准。

## 附录 B：daisyUI 5.8.2 约束

见 §3 第三条。

## 附录 C：本机环境约束（实施与验证时必须遵守）

- **8080 端口是用户自己的 nginx**（转发到内部站点），绝不能占用、结束或"清理"它。脚本化验证一律用 `127.0.0.1:18080` 及以上的端口。
- **仓库根目录的 `good-habit.db` 是用户的开发库**，不要打开、复制、删除或拿来登录。验证时新建临时库：`-db <临时目录>/preview.db`。
- **Go 工具链**：`C:/Users/Ryan/sdk/go1.27.4/bin/go.exe`（PATH 上的 `go` 是 1.20，太旧）。
- **不要运行 `build.sh` / `build.ps1`**（它们会先 `npm ci`，如果此时有 vite dev server 在运行，会把 `web/node_modules` 删成半残）。前端验证用 `npm run build` + Go 二进制直接托管产物，不启 vite dev server；如果确实需要 dev server，用完必须停掉，并确认端口已释放。
- **登录失败 5 次会锁账号 10 分钟**：验证数据一律通过接口新建（注册家长 → 建孩子 → 建积分项 / 奖品 → 登记几笔），不要对未知密码的账号反复尝试。
- **浏览器**：`browser-use` 连接的是用户正在使用的真实 Chrome，里面开着用户的工作标签页。只新建并操作自己的一个标签页，用完关掉；`Emulation.setDeviceMetricsOverride` 用完必须清除；`localStorage` 只能在应用自己的源上设置；Chrome 窗口被遮挡时动画帧会暂停，不要依赖 `requestAnimationFrame` 来判断"已加载"。
- **PowerShell**：不要用 `-ExecutionPolicy Bypass` 运行脚本（会被权限策略拒绝），需要时用 `powershell -NoProfile -Command "…"` 的单行内联命令。
- Bash 工具里不要用 heredoc 往仓库里写文件，用文件写入工具。

## 附录 D：样式源码

### D.1 `@theme` 新增内容

```css
@theme {
  /* ---- 字号刻度：每一档只担任一个角色 ---- */
  --text-hero: 4rem;
  --text-hero--line-height: 1;
  --text-hero--font-weight: 700;
  --text-hero--letter-spacing: -0.02em;
  --text-hero-lg: 4.5rem;
  --text-hero-lg--line-height: 1;
  --text-display: 2rem;
  --text-display--line-height: 1.2;
  --text-display--font-weight: 600;
  --text-display--letter-spacing: -0.01em;
  --text-stat: 1.75rem;
  --text-stat--line-height: 1.25;
  --text-stat--font-weight: 600;
  --text-title: 1.25rem;
  --text-title--line-height: 1.4;
  --text-title--font-weight: 600;

  /* ---- 阴影只有两层：浮层、孩子端主数字卡 ---- */
  --shadow-overlay: 0 16px 40px -12px rgb(43 39 34 / 0.28);
  --shadow-hero: 0 12px 28px -10px rgb(232 99 10 / 0.5);

  /* ---- 动效词汇表 ---- */
  --animate-pop: pop 0.28s cubic-bezier(0.34, 1.56, 0.64, 1);
  --animate-rise: rise 0.22s ease-out both;
  --animate-float-up: float-up 0.7s ease-out both;
  --animate-bump: bump 0.32s ease-out;
  --animate-wiggle: wiggle 0.5s ease-in-out 2;

  @keyframes pop { from { transform: scale(0.7); opacity: 0; } }
  @keyframes rise { from { opacity: 0; transform: translateY(6px); } }
  @keyframes float-up {
    from { opacity: 1; transform: translate(-50%, 0); }
    to { opacity: 0; transform: translate(-50%, -28px); }
  }
  @keyframes bump { 0%, 100% { transform: scale(1); } 35% { transform: scale(1.06); } }
  @keyframes wiggle { 0%, 100% { transform: rotate(0); } 25% { transform: rotate(-8deg); } 75% { transform: rotate(8deg); } }
}
```

（`--animate-pop` 与 `pop` 现有，原样保留；`--font-sans`、`--color-brand-ink` 及其深色覆盖也原样保留。）

### D.2 `@layer components` 新增内容

```css
@layer components {
  /* 实心橙底白字对比度仅 3.4:1：按钮统一 16px 半粗（沿用既有决策） */
  .btn { --fontsize: 1rem; }
  /* 浅色橙字一律用 brand-ink；soft 变体默认拿 primary 当前景 */
  .btn-soft.btn-primary {
    --btn-rest-fg: var(--color-brand-ink);
  }

  /* ---- 表面 ---- */
  .surface { @apply rounded-2xl border border-base-300 bg-base-100; }
  .inset { @apply rounded-2xl bg-base-200; }
  .divided > * + * { @apply border-t border-base-300/80; }

  /* 清单进场：逐行上浮，最多错开 8 行 */
  .stagger > * { @apply animate-rise; }
  .stagger > *:nth-child(2) { animation-delay: 30ms; }
  .stagger > *:nth-child(3) { animation-delay: 60ms; }
  .stagger > *:nth-child(4) { animation-delay: 90ms; }
  .stagger > *:nth-child(5) { animation-delay: 120ms; }
  .stagger > *:nth-child(6) { animation-delay: 150ms; }
  .stagger > *:nth-child(7) { animation-delay: 180ms; }
  .stagger > *:nth-child(n + 8) { animation-delay: 210ms; }

  .field-label { @apply mb-1.5 block text-sm font-medium text-base-content/70; }

  /* 状态胶囊：状态色只出现在圆点上，文字始终用文字色 */
  .status-chip {
    @apply inline-flex h-6 items-center gap-1.5 rounded-full bg-base-200 px-2.5 text-xs font-medium whitespace-nowrap text-base-content/75;
  }

  /* 浮动的 ±N：放在包住触发元素的定位容器里（.btn 会裁掉溢出内容） */
  .float-chip {
    @apply pointer-events-none absolute top-1 left-1/2 z-10 rounded-full bg-success px-2 py-0.5 text-sm font-semibold text-success-content;
    animation: var(--animate-float-up);
  }

  /* 路由过渡：新页面上浮进场 */
  .page-enter-active { animation: var(--animate-rise); }
}
```

（`--animate-float-up` 的关键帧用 `translate(-50%, …)` 居中，所以 `.float-chip` 只设 `left-1/2`，不要再加平移工具类。）
