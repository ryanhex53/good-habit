# 好习惯 · good-habit

家长为孩子登记好习惯赚积分，孩子用积分转盘抽奖的家庭小站。
**一个静态二进制文件就是全部**：前端、SQL 迁移都打包在内，零外部中间件（无 Docker / Redis / PostgreSQL），数据库是嵌入式 SQLite，灾备由 Litestream 流式同步到 Cloudflare R2。

## 功能一览

- **家长端**：注册账号 → 为孩子开子账号 → 维护积分项（如"托管课写完作业 +7"）→ 一键导入示例模板 → 逐笔登记得分、手动调整
- **孩子端**：独立登录 → 看余额与得分指南 → 玩 **Canvas 转盘**（单抽 / 保险抽：多付积分抽两次、挑满意的那个）→ 翻积分账单
- **账单**：每笔记录原因、±积分、所得奖品、**当时余额快照**，余额永不为负
- **奖品发放**：孩子中奖后家长在后台标记"已发放"
- 桌面与手机响应式，全中文界面；深浅双主题（跟随系统 + 手动切换，Tailwind CSS 4 + daisyUI 5）

## 架构速览

| 关注点 | 做法 |
|---|---|
| 存储 | SQLite（modernc.org/sqlite，纯 Go 无 CGO），WAL 模式 |
| 连接池 | ReadDB（`mode=ro`+`query_only`，5 连接并发读）/ WriteDB（`_txlock=immediate`，单连接 + 互斥锁）显式拆分 |
| 热路径 | 孩子余额、配置快照、会话、限流计数常驻内存（细粒度锁 + atomic），抽奖/登记不穿透磁盘 |
| 落盘 | 写入事件进 8192 缓冲 channel，单写协程按 **64 条 / 15ms** 组提交；SIGTERM 优雅停机 Drain 零丢失 |
| 容灾 | `wal_autocheckpoint=0` 交给 Litestream（10s 增量 → R2），配套 restore 脚本 |

完整设计见 [docs/superpowers/specs/2026-11-25-good-habit-design.md](docs/superpowers/specs/2026-11-25-good-habit-design.md)（含数据丢失窗口的如实声明：kill -9 最多丢约 15ms/64 条，整机灾难最多丢约 10 秒）。

## 快速开始（开发）

要求：Go ≥ 1.26（modernc 依赖底线）、Node ≥ 20。

```bash
# 后端（本机 127.0.0.1:8080，数据文件 good-habit.db）
go run ./cmd/good-habit

# 前端（另一个终端，Vite 热更新，/api 自动代理到 8080）
cd web && npm install && npm run dev
```

打开 Vite 打印的地址 → 注册一个家长账号，到「孩子 / 积分项 / 奖品」页配几条数据即可体验全流程（示例模板可经 `POST /api/bootstrap` 导入，需登录会话）。

## 构建 / 部署

```bash
bash build.sh        # Windows 用 .\build.ps1；产出 dist/good-habit-linux-amd64（约 12MB 静态二进制）
```

服务器部署（systemd、R2 备份、恢复演练、TLS 前置）完整步骤见 **[deploy/README.md](deploy/README.md)**。

忘记密码：`good-habit reset-password -user <用户名> -new <新密码>`（服务器本地执行，重置后需 `systemctl restart good-habit`）。

## 目录结构

```
cmd/good-habit/     入口（服务 + reset-password 子命令）
internal/
  app/              装配与优雅停机
  db/               读写池、PRAGMA 基线、迁移执行器
  ledger/           组提交落盘算子
  writer/           8192 队列 + 64 条/15ms 批量提交 + Flush 屏障
  store/            内存热路径（余额/快照/会话/计数）
  auth/             bcrypt、会话、登录锁定
  ratelimit/        内存定长窗口限流
  httpapi/          全部 HTTP 端点（含静态资源 SPA 服务）
  sweep/            48h 超时待选局清扫
  seed/             附录 A 示例数据
  webui/            前端构建产物 embed（dist/）
web/                Vue 3 + TypeScript + Vite + Tailwind/daisyUI 源码
migrations/         SQL 迁移（embed 进二进制）
deploy/             systemd / Litestream / restore 脚本 / 部署文档
```

## 测试

```bash
go test ./... -timeout 300s        # 12 个包：单测 + 全旅程集成（真实 SQLite，无 mock）
cd web && npm run build            # vue-tsc 类型检查 + 构建（Tailwind/daisyUI 编译产物）
```

并发正确性另有 `-race` 建议在 Linux/CI 侧执行（Windows 无 gcc 时不可用；测试本身已用精确计数断言兜底）。

## 文档索引

- [设计文档](docs/superpowers/specs/2026-11-25-good-habit-design.md)——产品行为、架构约束、不变量、数据窗口
- [实施计划](docs/superpowers/plans/2026-11-25-good-habit.md)——24 个 TDD 任务的执行记录
- [部署指南](deploy/README.md)——systemd / R2 备份 / 恢复演练 / TLS
