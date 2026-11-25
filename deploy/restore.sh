#!/usr/bin/env bash
# 一键恢复：从 Cloudflare R2 恢复 good-habit 数据库并重启服务。
# 用法：
#   ./restore.sh                 # 恢复到最新副本
#   ./restore.sh -timestamp "2026-11-25T12:00:00Z"   # 恢复到指定时间点（litestream 支持的格式）
set -euo pipefail

TS_ARG=""
if [ "${1:-}" = "-timestamp" ] && [ -n "${2:-}" ]; then
  TS_ARG="-timestamp ${2}"
fi

DB_PATH=/var/lib/good-habit/app.db
CFG=/etc/good-habit/litestream.yml
SYSTEMD_UNIT=good-habit.service

echo "==> 停止应用（先优雅 Drain）"
systemctl stop "$SYSTEMD_UNIT"

echo "==> 备份现有库（若存在）"
if [ -f "$DB_PATH" ]; then
  cp "$DB_PATH" "$DB_PATH.pre-restore.$(date +%Y%m%d%H%M%S)"
  rm -f "$DB_PATH" "$DB_PATH-wal" "$DB_PATH-shm"
  echo "    已备份为 $DB_PATH.pre-restore.*"
fi

echo "==> 从 R2 恢复 ${TS_ARG:-（最新）}"
litestream restore -config "$CFG" $TS_ARG -o "$DB_PATH"

echo "==> 校验"
ls -la "$DB_PATH"

echo "==> 重启应用"
systemctl start "$SYSTEMD_UNIT"
sleep 2
systemctl is-active "$SYSTEMD_UNIT" && echo "恢复完成 ✔" || {
  echo "应用未起来，请查 journalctl -u $SYSTEMD_UNIT" >&2
  exit 1
}
