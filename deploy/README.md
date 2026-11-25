# good-habit 部署指南

单二进制 + SQLite + Litestream(R2) 的单机部署。应用零外部中间件：没有 Docker、
没有 Redis/PostgreSQL，所有东西（前端、迁移）都打进一个可执行文件。

## 1. 服务器准备（Linux amd64）

```bash
# 建专用用户与数据目录
sudo useradd --system --home /var/lib/good-habit --shell /usr/sbin/nologin good-habit
sudo mkdir -p /var/lib/good-habit
sudo chown good-habit:good-habit /var/lib/good-habit
sudo mkdir -p /etc/good-habit
```

需要：Go ≥ 1.26（仅构建期需要；modernc/sqlite v1.63 的底线，README 原写 1.23；也可在开发机交叉编译后只传二进制）、Node ≥ 20
（仅构建期）、Litestream v0.5+（运行期，见 [litestream.io](https://litestream.io)）。

## 2. 构建

在开发机（Windows/Linux/macOS 均可）仓库根目录：

```bash
bash build.sh        # 产出 dist/good-habit-linux-amd64（约 11MB，静态链接）
# Windows PowerShell:  .\build.ps1
```

脚本会先 `npm ci && npm run build` 构建前端到 `internal/webui/dist`，再以
`CGO_ENABLED=0` 交叉编译（驱动为 modernc.org/sqlite 纯 Go，无 CGO）。
在 Linux 服务器上直接 `go build` 也可以。

## 3. 安装

```bash
sudo install -m 0755 dist/good-habit-linux-amd64 /usr/local/bin/good-habit
sudo install -m 0644 deploy/good-habit.service /etc/systemd/system/
sudo install -m 0644 deploy/litestream.service  /etc/systemd/system/
sudo install -m 0644 deploy/litestream.yml      /etc/good-habit/
sudo install -m 0700 deploy/restore.sh          /usr/local/bin/good-habit-restore
sudo systemctl daemon-reload
sudo systemctl enable --now good-habit
```

## 4. Cloudflare R2 备份

1. Cloudflare 控制台建桶（如 `good-habit`），生成 API Token（Object Read & Write）。
2. 编辑 `/etc/good-habit/litestream.yml`：把 `<ACCOUNT_ID>` 换成你的账户 ID、桶名。
3. 写密钥文件（勿入库勿入 git）：

```bash
sudo tee /etc/good-habit/litestream.env >/dev/null <<'EOF'
LITESTREAM_ACCESS_KEY_ID=<你的 R2 Access Key ID>
LITESTREAM_SECRET_ACCESS_KEY=<你的 R2 Secret Access Key>
EOF
sudo chmod 600 /etc/good-habit/litestream.env
sudo systemctl enable --now litestream
```

Litestream 每 10 秒增量同步 WAL；应用侧已设置 `PRAGMA wal_autocheckpoint=0`，
checkpoint 完全由 Litestream 管理，保证增量链不断。

## 5. TLS / 对外暴露

应用本身只说 HTTP，默认绑 `127.0.0.1:8080`。公网使用必须前置 TLS（任选其一）：

- Cloudflare Tunnel（`cloudflared`，免开端口）；
- Caddy / nginx 反代到 127.0.0.1:8080。

前置代理时开启信任代理头：

```bash
# systemd unit 的 ExecStart 加环境变量，或用 EnvironmentFile：
Environment=GOODHABIT_TRUST_PROXY=1
Environment=GOODHABIT_SECURE_COOKIE=1
```

## 6. 日常运维

```bash
systemctl status good-habit litestream
journalctl -u good-habit -f          # 访问日志（含请求 ID）
good-habit reset-password -user <用户名> -new <新密码>   # 忘记密码（服务器本地执行）
```

## 7. 恢复演练（建议每月做一次）

```bash
sudo good-habit-restore                       # 恢复到最新
sudo good-habit-restore -timestamp "2026-11-25T12:00:00Z"   # 恢复到时间点
```

脚本会：停应用 → 备份现有库 → `litestream restore` → 起应用。

## 8. 数据窗口说明（如实告知）

- 优雅停机（SIGTERM/systemctl stop）：写队列 Drain，**零丢失**。
- 硬崩（kill -9 / 断电）：最多丢约 **15ms / 64 条**的未提交写入（架构取舍，
  见设计文档 §9）。
- 整机灾难：R2 副本落后主库 ≤ **10 秒** + 一个组提交窗口。

## 9. 首次启动引导

注册第一个家长账号后，总览页提供「一键导入示例」（11 个积分项 + 3 个奖品 +
5/7 定价），可再按需修改。

## 运维补充

- `good-habit reset-password` 只清数据库中的会话行；**正在运行的服务持有内存会话**，重置后请执行
  `systemctl restart good-habit` 使旧会话彻底失效。
- `GOODHABIT_TRUST_PROXY=1` 时的真实 IP 取值：优先 `CF-Connecting-IP`（Cloudflare Tunnel），
  否则取 `X-Forwarded-For` 最右侧一跳（首段可被客户端伪造，不可用作限流依据）。
