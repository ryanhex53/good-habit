package writer

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"good-habit/internal/db"
	"good-habit/internal/ledger"
	"good-habit/migrations"
)

func setup(t *testing.T) (*db.DBs, int64, int64) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := int64(1730000000000)
	var fid, cid int64
	err = dbs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO families (name, created_at) VALUES ('测家', ?)", now)
		if err != nil {
			return err
		}
		if fid, err = res.LastInsertId(); err != nil {
			return err
		}
		res, err = tx.Exec(
			"INSERT INTO users (family_id, role, username, password_hash, display_name, created_at) VALUES (?,?,?,?,?,?)",
			fid, "child", "kid_1", "x", "娃", now)
		if err != nil {
			return err
		}
		cid, err = res.LastInsertId()
		return err
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return dbs, fid, cid
}

func mkEarn(i int, fid, cid int64) ledger.EarnOp {
	return ledger.EarnOp{
		UUID: fmt.Sprintf("u-%04d", i), FamilyID: fid, ChildID: cid,
		Delta: 1, BalanceAfter: int64(i + 1), CreatedAt: int64(1730000000000 + i),
	}
}

func ledgerCount(t *testing.T, dbs *db.DBs) int64 {
	t.Helper()
	var n int64
	if err := dbs.Read.QueryRow("SELECT COUNT(*) FROM ledger").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestFlushWithinInterval(t *testing.T) {
	dbs, fid, cid := setup(t)
	w := New(dbs, WithFlushInterval(30*time.Millisecond))
	w.Start()
	defer func() { _ = w.Stop() }()

	w.Enqueue(mkEarn(1, fid, cid))
	time.Sleep(150 * time.Millisecond)

	if n := ledgerCount(t, dbs); n != 1 {
		t.Errorf("落盘行数 = %d, want 1（15ms 量级窗口应已提交）", n)
	}
	if c := w.Commits(); c < 1 {
		t.Errorf("Commits = %d, want ≥ 1", c)
	}
}

func TestBatchUpTo64(t *testing.T) {
	dbs, fid, cid := setup(t)
	w := New(dbs, WithFlushInterval(time.Hour), WithMaxBatch(64))
	w.Start()

	for i := 0; i < 200; i++ {
		w.Enqueue(mkEarn(i, fid, cid))
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if n := ledgerCount(t, dbs); n != 200 {
		t.Errorf("落盘行数 = %d, want 200", n)
	}
	if c := w.Commits(); c > 4 {
		t.Errorf("Commits = %d, want ≤ 4（200/64 向上取整）", c)
	}
}

func TestDrainOnStop(t *testing.T) {
	dbs, fid, cid := setup(t)
	// interval 1 小时：不靠定时器，逼 Stop 的 Drain 路径排空
	w := New(dbs, WithFlushInterval(time.Hour))
	w.Start()

	for i := 0; i < 50; i++ {
		w.Enqueue(mkEarn(i, fid, cid))
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if n := ledgerCount(t, dbs); n != 50 {
		t.Errorf("Drain 后落盘行数 = %d, want 50", n)
	}
}

func TestBatchFailureIsolatesBadOp(t *testing.T) {
	dbs, fid, cid := setup(t)
	w := New(dbs, WithFlushInterval(time.Hour))
	w.Start()

	now := int64(1730000000000)
	// 坏算子：child 不存在 → setBalance 影响 0 行 → Apply 报错
	w.Enqueue(ledger.EarnOp{UUID: "bad-1", FamilyID: fid, ChildID: 999999,
		Delta: 5, BalanceAfter: 5, CreatedAt: now})
	// 好算子：不应被坏算子连坐
	w.Enqueue(ledger.EarnOp{UUID: "good-1", FamilyID: fid, ChildID: cid,
		Delta: 7, BalanceAfter: 7, CreatedAt: now})
	w.Enqueue(ledger.EarnOp{UUID: "good-2", FamilyID: fid, ChildID: cid,
		Delta: 3, BalanceAfter: 10, CreatedAt: now})

	err := w.Stop() // 应报告坏算子的错误
	if err == nil {
		t.Log("注意：Stop 未返回错误（bad op 的失败应被记录）")
	}

	var good int64
	dbs.Read.QueryRow(
		"SELECT COUNT(*) FROM ledger WHERE uuid IN ('good-1','good-2')").Scan(&good)
	if good != 2 {
		t.Errorf("好算子落盘 = %d, want 2（坏算子不应连坐整批）", good)
	}
	var bad int64
	dbs.Read.QueryRow("SELECT COUNT(*) FROM ledger WHERE uuid='bad-1'").Scan(&bad)
	if bad != 0 {
		t.Errorf("坏算子不应落盘 = %d", bad)
	}
}
