package sweep

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"good-habit/internal/db"
	"good-habit/internal/store"
	"good-habit/migrations"
)

func newDB(t *testing.T) *db.DBs {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return dbs
}

func TestSweepPending(t *testing.T) {
	dbs := newDB(t)
	now := time.Now().UnixMilli()
	old := now - 49*3600*1000 // 49 小时前
	uuid := "sweep-uuid-1"

	var fid, cid, pz int64
	err := dbs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO families (name, created_at) VALUES ('扫家', ?)", now)
		if err != nil {
			return err
		}
		if fid, err = res.LastInsertId(); err != nil {
			return err
		}
		res, err = tx.Exec(
			"INSERT INTO users (family_id, role, username, password_hash, display_name, balance, created_at) VALUES (?,?,?,?,?,?,?)",
			fid, "child", "sweepkid", "h", "娃", 93, now)
		if err != nil {
			return err
		}
		if cid, err = res.LastInsertId(); err != nil {
			return err
		}
		res, err = tx.Exec("INSERT INTO prizes (family_id, name, created_at) VALUES (?,?,?)", fid, "糖", now)
		if err != nil {
			return err
		}
		if pz, err = res.LastInsertId(); err != nil {
			return err
		}
		_, err = tx.Exec(
			`INSERT INTO draws (uuid, family_id, child_id, mode, cost, prize0_id, prize1_id, picked, state, created_at)
			 VALUES (?,?,?,?,?,?,?,NULL,'pending_choice',?)`,
			uuid, fid, cid, "double", 7, pz, pz, old)
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	st := store.New()
	c := st.Child(cid)
	c.Lock()
	c.Balance = 93
	c.PendingUUID = uuid
	c.PendingDeadline = old + 48*3600*1000
	c.Unlock()

	n, err := Run(dbs, st)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 1 {
		t.Fatalf("清扫数 = %d, want 1", n)
	}

	var picked int64
	var state string
	if err := dbs.Read.QueryRow(
		"SELECT picked, state FROM draws WHERE uuid=?", uuid,
	).Scan(&picked, &state); err != nil || picked != 0 || state != "completed" {
		t.Fatalf("清扫后落盘 = picked %d state %s err=%v, want 0/completed", picked, state, err)
	}
	bal, pending, unful := st.Child(cid).Stats()
	if bal != 93 || pending != "" || unful != 1 {
		t.Errorf("清扫后内存 = %d/%q/%d, want 93/\"\"/1", bal, pending, unful)
	}
}

func TestSweepNoop(t *testing.T) {
	dbs := newDB(t)
	st := store.New()
	n, err := Run(dbs, st)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n != 0 {
		t.Errorf("空库清扫数 = %d, want 0", n)
	}
}
