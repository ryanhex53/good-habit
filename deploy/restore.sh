#!/usr/bin/env bash
# 一键恢复：从 Cloudflare R2 恢复 good-habit 数据库并重启服务。
# 用法：
#   ./restore.sh                                  # 恢复到最新副本（交互确认）
#   ./restore.sh -timestamp "2026-11-25T12:00:00Z"  # 恢复到指定时间点
#   FORCE=1 ./restore.sh ...                       # 跳过确认（自动化场景）
set -euo pipefail

TS_ARG=""
if [ "${1:-}" = "-timestamp" ] && [ -n "${2:-}" ]; then
  TS_ARG="-timestamp ${2}"
fi

DB_PATH=/var/lib/good-habit/app.db
CFG=/etc/good-habit/litestream.yml
APP_UNIT=good-habit.service
LS_UNIT=litestream.service

echo "==> 停止应用与 Litestream（先优雅 Drain）"
systemctl stop "$APP_UNIT"
systemctl stop "$LS_UNIT" 2>/dev/null || true

echo "==> 备份现有库（app.db + WAL + SHM 三件一起，WAL 里可能有未 checkpoint 的已提交数据）"
STAMP=$(date +%Y%m%d%H%M%S)
for ext in "" "-wal" "-shm"; do
  if [ -f "$DB_PATH$ext" ]; then
    cp "$DB_PATH$ext" "$DB_PATH$ext.pre-restore.$STAMP"
    echo "    已备份 $DB_PATH$ext"
    rm -f "$DB_PATH$ext"
  fi
done

if [ -z "${FORCE:-}" ] && [ -f "$DB_PATH.pre-restore.$STAMP" ]; then
  read -r -p "现有库已备份为 .pre-restore.$STAMP，确认从 R2 覆盖恢复？[y/N] " ans
  case "$ans" in
    y|Y) ;;
    *) echo "已取消。原库仍可从 .pre-restore.$STAMP 恢复。"; exit 1 ;;
  esac
fi

echo "==> 从 R2 恢复 ${TS_ARG:-（最新）}"
litestream restore -config "$CFG" $TS_ARG -o "$DB_PATH"

echo "==> 校验"
ls -la "$DB_PATH"
if command -v sqlite3 >/dev/null 2>&1; then
  if sqlite3 "$DB_PATH" "PRAGMA integrity_check;" | grep -q '^ok$'; then
    echo "    integrity_check: ok"
  else
    echo "    integrity_check 失败！原库备份在 $DB_PATH.pre-restore.$STAMP" >&2
    exit 1
  fi
else
  echo "    （未安装 sqlite3，跳过 integrity_check）"
fi

echo "==> 重启应用与 Litestream"
systemctl start "$APP_UNIT"
systemctl start "$LS_UNIT"
sleep 2
systemctl is-active "$APP_UNIT" >/dev/null && echo "恢复完成 ✔" || {
  echo "应用未起来，请查 journalctl -u $APP_UNIT（原库备份在 $DB_PATH.pre-restore.$STAMP）" >&2
  exit 1
}
