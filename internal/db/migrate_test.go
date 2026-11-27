package db

import (
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
	if n != 2 {
		t.Errorf("schema_migrations 行数 = %d, want 2", n)
	}
}

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
