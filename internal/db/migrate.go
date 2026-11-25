package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migrate 按文件名序执行未应用的迁移：每个迁移单独一个事务（WithTx），
// 执行成功后在 schema_migrations 记账；已应用的跳过，可安全重复调用。
// schema_migrations 由 0001 建表；首次运行前查询失败视为空记录。
func (d *DBs) Migrate(fsys fs.FS) error {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("glob 迁移文件: %w", err)
	}
	sort.Strings(names)

	applied := map[int64]bool{}
	rows, err := d.Read.Query("SELECT version FROM schema_migrations")
	if err == nil {
		for rows.Next() {
			var v int64
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return fmt.Errorf("读迁移记录: %w", err)
			}
			applied[v] = true
		}
		rows.Close()
	}

	for _, name := range names {
		v, err := versionOf(name)
		if err != nil {
			return fmt.Errorf("迁移文件名 %q: %w", name, err)
		}
		if applied[v] {
			continue
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("读迁移 %s: %w", name, err)
		}
		err = d.WithTx(func(tx *sql.Tx) error {
			if _, err := tx.Exec(string(body)); err != nil {
				return fmt.Errorf("执行 %s: %w", name, err)
			}
			_, err := tx.Exec(
				"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
				v, time.Now().UnixMilli(),
			)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// versionOf 从 "0001_init.sql" 这类文件名解析出迁移版本号。
func versionOf(name string) (int64, error) {
	i := strings.IndexByte(name, '_')
	if i <= 0 {
		return 0, fmt.Errorf("缺版本前缀（应为 <数字>_<名称>.sql）")
	}
	return strconv.ParseInt(name[:i], 10, 64)
}
