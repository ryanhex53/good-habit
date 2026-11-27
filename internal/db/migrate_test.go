package db

import (
	"database/sql"
	"testing"
	"testing/fstest"

	"good-habit/migrations"
)

func TestMigrateFresh(t *testing.T) {
	dbs := open(t)
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, table := range []string{
		"families", "users", "reward_items", "reward_item_children", "prizes",
		"draws", "ledger", "sessions", "schema_migrations",
	} {
		var name string
		err := dbs.Read.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("表 %s 应已创建: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	dbs := open(t)
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("第一次 Migrate: %v", err)
	}
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("第二次 Migrate 应幂等: %v", err)
	}
	var n int64
	if err := dbs.Read.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("查 schema_migrations: %v", err)
	}
	if n != 3 {
		t.Errorf("schema_migrations 行数 = %d, want 3", n)
	}
}

// TestMigrateFromV1：线上库停在 0001 时，升级必须原地补上后续迁移。
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
		t.Fatalf("升级到最新: %v", err)
	}
	var n int64
	dbs.Read.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n)
	if n != 3 {
		t.Errorf("schema_migrations 行数 = %d, want 3", n)
	}
	var name string
	if err := dbs.Read.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='reward_item_children'",
	).Scan(&name); err != nil {
		t.Errorf("reward_item_children 应已创建: %v", err)
	}
}

// TestMigrateFromV2AddsUserAgent：线上库停在 0002 时，升级必须原地给 ledger 补上 user_agent 列
// （仓库里首个 ALTER TABLE ... ADD COLUMN），历史行读出空串。
func TestMigrateFromV2AddsUserAgent(t *testing.T) {
	dbs := open(t)
	old := fstest.MapFS{}
	for _, name := range []string{"0001_init.sql", "0002_reward_item_children.sql"} {
		b, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("读 %s: %v", name, err)
		}
		old[name] = &fstest.MapFile{Data: b}
	}
	if err := dbs.Migrate(old); err != nil {
		t.Fatalf("迁到 0002: %v", err)
	}
	// 按 0002 的旧列清单铺一行账单（没有 user_agent）
	now := int64(1730000000000)
	err := dbs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO families (name, created_at) VALUES ('老家', ?)`, now)
		if err != nil {
			return err
		}
		fid, _ := res.LastInsertId()
		res, err = tx.Exec(`INSERT INTO users (family_id, role, username, password_hash, display_name, created_at)
			VALUES (?, 'child', 'oldkid', 'x', '老娃', ?)`, fid, now)
		if err != nil {
			return err
		}
		cid, _ := res.LastInsertId()
		_, err = tx.Exec(`INSERT INTO ledger (uuid, family_id, child_id, delta, balance_after, kind, created_at)
			VALUES ('old-1', ?, ?, 5, 5, 'earn', ?)`, fid, cid, now)
		return err
	})
	if err != nil {
		t.Fatalf("铺旧行: %v", err)
	}

	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("升级到最新: %v", err)
	}
	var n int64
	dbs.Read.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n)
	if n != 3 {
		t.Errorf("schema_migrations 行数 = %d, want 3", n)
	}
	var typ string
	var notnull int
	var dflt sql.NullString
	err = dbs.Read.QueryRow(
		"SELECT type, [notnull], dflt_value FROM pragma_table_info('ledger') WHERE name = 'user_agent'",
	).Scan(&typ, &notnull, &dflt)
	if err != nil {
		t.Fatalf("user_agent 列应已存在: %v", err)
	}
	if typ != "TEXT" || notnull != 1 || dflt.String != "''" {
		t.Errorf("user_agent 定义 = type=%s notnull=%d dflt=%q, want TEXT / 1 / ''", typ, notnull, dflt.String)
	}
	var ua string
	if err := dbs.Read.QueryRow("SELECT user_agent FROM ledger WHERE uuid = 'old-1'").Scan(&ua); err != nil {
		t.Fatalf("读旧行 user_agent: %v", err)
	}
	if ua != "" {
		t.Errorf("旧行 user_agent = %q, want 空串", ua)
	}
}
