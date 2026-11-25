package db

import (
	"testing"

	"good-habit/migrations"
)

func TestMigrateFresh(t *testing.T) {
	dbs := open(t)
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for _, table := range []string{
		"families", "users", "reward_items", "prizes",
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
	if n != 1 {
		t.Errorf("schema_migrations 行数 = %d, want 1", n)
	}
}
