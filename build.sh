#!/usr/bin/env bash
# 构建单个 Linux 静态二进制（前端 + 后端 + 迁移全 embed）。
# 用法：bash build.sh   → 产出 dist/good-habit-linux-amd64
set -euo pipefail
cd "$(dirname "$0")"

# ---- 挑 Go ≥1.23 工具链：优先 PATH，其次 ~/sdk 里最新的 ----
GO_BIN="$(command -v go 2>/dev/null || true)"
if compgen -G "$HOME"/sdk/go1.*/bin/go >/dev/null 2>&1; then
  GO_BIN="$(ls -1d "$HOME"/sdk/go1.*/bin/go | sort -V | tail -1)"
fi
if [ -z "$GO_BIN" ]; then
  echo "错误：找不到 go，需要 Go ≥1.26" >&2
  exit 1
fi
echo "使用 $("$GO_BIN" version)"

# ---- 前端 ----
( cd web && npm ci && npm run build )

# ---- 后端（纯 Go 无 CGO，交叉编译即静态） ----
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$GO_BIN" build \
  -trimpath -ldflags='-s -w' \
  -o dist/good-habit-linux-amd64 ./cmd/good-habit

echo "构建完成: dist/good-habit-linux-amd64"
