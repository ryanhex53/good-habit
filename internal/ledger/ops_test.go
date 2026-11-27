package ledger

import (
	"database/sql"
	"path/filepath"
	"testing"

	"good-habit/internal/db"
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

// seed 插入最小夹具：family + parent + child + 2 个积分项 + 2 个奖品。
type fixture struct {
	familyID, parentID, childID, itemID, prize0ID, prize1ID int64
}

func seed(t *testing.T, dbs *db.DBs) fixture {
	t.Helper()
	now := int64(1730000000000)
	var f fixture
	err := dbs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec("INSERT INTO families (name, created_at) VALUES ('测家', ?)", now)
		if err != nil {
			return err
		}
		if f.familyID, err = res.LastInsertId(); err != nil {
			return err
		}
		for _, u := range []struct {
			role, name string
			uid        *int64
		}{{"parent", "爸", &f.parentID}, {"child", "娃", &f.childID}} {
			res, err = tx.Exec(
				"INSERT INTO users (family_id, role, username, password_hash, display_name, created_at) VALUES (?,?,?,?,?,?)",
				f.familyID, u.role, u.role+"_1", "x", u.name, now)
			if err != nil {
				return err
			}
			if *u.uid, err = res.LastInsertId(); err != nil {
				return err
			}
		}
		for _, it := range []struct {
			title  string
			points int64
			uid    *int64
		}{{"托管课写完作业", 7, &f.itemID}} {
			res, err = tx.Exec(
				"INSERT INTO reward_items (family_id, title, points, created_at) VALUES (?,?,?,?)",
				f.familyID, it.title, it.points, now)
			if err != nil {
				return err
			}
			if *it.uid, err = res.LastInsertId(); err != nil {
				return err
			}
		}
		for _, pz := range []struct {
			name string
			uid  *int64
		}{{"水果糖1颗", &f.prize0ID}, {"巧克力1颗", &f.prize1ID}} {
			res, err = tx.Exec(
				"INSERT INTO prizes (family_id, name, created_at) VALUES (?,?,?)",
				f.familyID, pz.name, now)
			if err != nil {
				return err
			}
			if *pz.uid, err = res.LastInsertId(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return f
}

func TestEarnOpApplies(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)
	now := int64(1730000001000)

	op := EarnOp{
		UUID: "u-earn-1", FamilyID: f.familyID, ChildID: f.childID,
		RewardItemID: f.itemID, Delta: 7, BalanceAfter: 7, UserAgent: "ua-earn", CreatedAt: now,
	}
	if err := dbs.WithTx(op.Apply); err != nil {
		t.Fatalf("EarnOp.Apply: %v", err)
	}

	var uuid, kind, ua string
	var delta, bal, refItem, createdAt int64
	var note sql.NullString
	err := dbs.Read.QueryRow(
		"SELECT uuid, kind, delta, balance_after, ref_reward_item_id, note, user_agent, created_at FROM ledger WHERE uuid = ?",
		"u-earn-1",
	).Scan(&uuid, &kind, &delta, &bal, &refItem, &note, &ua, &createdAt)
	if err != nil {
		t.Fatalf("查 ledger: %v", err)
	}
	if uuid != "u-earn-1" || kind != "earn" || delta != 7 || bal != 7 ||
		refItem != f.itemID || note.Valid || ua != "ua-earn" || createdAt != now {
		t.Errorf("ledger 行不符: %+v", op)
	}
	var userBal int64
	if err := dbs.Read.QueryRow("SELECT balance FROM users WHERE id=?", f.childID).Scan(&userBal); err != nil {
		t.Fatalf("查 users: %v", err)
	}
	if userBal != 7 {
		t.Errorf("users.balance = %d, want 7", userBal)
	}

	// 空 UA 直接存空串（不能用 nullString：NOT NULL 列会让整个 op 失败）
	op2 := EarnOp{UUID: "u-earn-2", FamilyID: f.familyID, ChildID: f.childID, RewardItemID: f.itemID, Delta: 1, BalanceAfter: 8, CreatedAt: now}
	if err := dbs.WithTx(op2.Apply); err != nil {
		t.Fatalf("EarnOp.Apply 空 UA: %v", err)
	}
	var ua2 string
	if err := dbs.Read.QueryRow("SELECT user_agent FROM ledger WHERE uuid = 'u-earn-2'").Scan(&ua2); err != nil {
		t.Fatalf("查空 UA 行: %v", err)
	}
	if ua2 != "" {
		t.Errorf("空 UA 行 user_agent = %q, want 空串", ua2)
	}
}

func TestAdjustOpApplies(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)
	now := int64(1730000003000)

	op := AdjustOp{
		UUID: "u-adj-1", FamilyID: f.familyID, ChildID: f.childID,
		Delta: -3, BalanceAfter: -3, Note: "测试", UserAgent: "ua-adj", CreatedAt: now,
	}
	if err := dbs.WithTx(op.Apply); err != nil {
		t.Fatalf("AdjustOp.Apply: %v", err)
	}
	var uuid, kind, ua string
	var note sql.NullString
	var delta, bal, createdAt int64
	err := dbs.Read.QueryRow(
		"SELECT uuid, kind, delta, balance_after, note, user_agent, created_at FROM ledger WHERE uuid = 'u-adj-1'",
	).Scan(&uuid, &kind, &delta, &bal, &note, &ua, &createdAt)
	if err != nil {
		t.Fatalf("查 ledger: %v", err)
	}
	if uuid != "u-adj-1" || kind != "adjust" || delta != -3 || bal != -3 ||
		note.String != "测试" || ua != "ua-adj" || createdAt != now {
		t.Errorf("ledger 行不符: %+v", op)
	}
	var userBal int64
	if err := dbs.Read.QueryRow("SELECT balance FROM users WHERE id=?", f.childID).Scan(&userBal); err != nil {
		t.Fatalf("查 users: %v", err)
	}
	if userBal != -3 {
		t.Errorf("users.balance = %d, want -3", userBal)
	}

	// 空 UA 直接存空串
	op2 := AdjustOp{UUID: "u-adj-2", FamilyID: f.familyID, ChildID: f.childID, Delta: 1, BalanceAfter: -2, Note: "x", CreatedAt: now}
	if err := dbs.WithTx(op2.Apply); err != nil {
		t.Fatalf("AdjustOp.Apply 空 UA: %v", err)
	}
	var ua2 string
	if err := dbs.Read.QueryRow("SELECT user_agent FROM ledger WHERE uuid = 'u-adj-2'").Scan(&ua2); err != nil {
		t.Fatalf("查空 UA 行: %v", err)
	}
	if ua2 != "" {
		t.Errorf("空 UA 行 user_agent = %q, want 空串", ua2)
	}
}

func TestDrawOps(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)
	now := int64(1730000002000)
	zero := 0

	// 单抽：立即 completed，picked=0
	single := DrawOp{
		UUID: "u-draw-1", FamilyID: f.familyID, ChildID: f.childID,
		Mode: "single", Cost: 5, Prize0ID: f.prize0ID,
		State: "completed", Picked: &zero,
		Delta: -5, BalanceAfter: 95, CreatedAt: now,
	}
	if err := dbs.WithTx(single.Apply); err != nil {
		t.Fatalf("single Apply: %v", err)
	}

	var drawID int64
	var picked sql.NullInt64
	var prize1, fulfilled sql.NullInt64
	var state, mode string
	err := dbs.Read.QueryRow(
		"SELECT id, mode, prize1_id, picked, state, fulfilled_at FROM draws WHERE uuid='u-draw-1'",
	).Scan(&drawID, &mode, &prize1, &picked, &state, &fulfilled)
	if err != nil {
		t.Fatalf("查 draws: %v", err)
	}
	if mode != "single" || state != "completed" || prize1.Valid || !picked.Valid || picked.Int64 != 0 || fulfilled.Valid {
		t.Errorf("single draws 行不符: mode=%s state=%s prize1=%v picked=%v fulfilled=%v", mode, state, prize1, picked, fulfilled)
	}

	var refDraw, delta, bal int64
	var kind string
	err = dbs.Read.QueryRow(
		"SELECT kind, delta, balance_after, ref_draw_id FROM ledger WHERE uuid='u-draw-1'",
	).Scan(&kind, &delta, &bal, &refDraw)
	if err != nil {
		t.Fatalf("查 ledger: %v", err)
	}
	if kind != "draw" || delta != -5 || bal != 95 || refDraw != drawID {
		t.Errorf("single ledger 行不符: kind=%s delta=%d bal=%d refDraw=%d(want %d)", kind, delta, bal, refDraw, drawID)
	}
	var userBal int64
	dbs.Read.QueryRow("SELECT balance FROM users WHERE id=?", f.childID).Scan(&userBal)
	if userBal != 95 {
		t.Errorf("users.balance = %d, want 95", userBal)
	}

	// 双抽：pending_choice，picked IS NULL，两个奖品都记录
	dbl := DrawOp{
		UUID: "u-draw-2", FamilyID: f.familyID, ChildID: f.childID,
		Mode: "double", Cost: 7, Prize0ID: f.prize0ID, Prize1ID: f.prize1ID,
		State: "pending_choice", Picked: nil,
		Delta: -7, BalanceAfter: 88, CreatedAt: now,
	}
	if err := dbs.WithTx(dbl.Apply); err != nil {
		t.Fatalf("double Apply: %v", err)
	}
	err = dbs.Read.QueryRow(
		"SELECT prize1_id, picked, state FROM draws WHERE uuid='u-draw-2'",
	).Scan(&prize1, &picked, &state)
	if err != nil {
		t.Fatalf("查 double draws: %v", err)
	}
	if !prize1.Valid || prize1.Int64 != f.prize1ID || picked.Valid || state != "pending_choice" {
		t.Errorf("double draws 行不符: prize1=%v picked=%v state=%s", prize1, picked, state)
	}
	dbs.Read.QueryRow("SELECT balance FROM users WHERE id=?", f.childID).Scan(&userBal)
	if userBal != 88 {
		t.Errorf("users.balance = %d, want 88", userBal)
	}
}

func TestSessionOps(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)
	now := int64(1730000003000)

	up := SessionUpsertOp{TokenHash: "hash-1", UserID: f.parentID, ExpiresAt: now + 86400000, CreatedAt: now}
	if err := dbs.WithTx(up.Apply); err != nil {
		t.Fatalf("SessionUpsertOp: %v", err)
	}
	var n int64
	dbs.Read.QueryRow("SELECT COUNT(*) FROM sessions WHERE token_hash='hash-1'").Scan(&n)
	if n != 1 {
		t.Fatalf("sessions 行数 = %d, want 1", n)
	}

	del := SessionDeleteOp{TokenHash: "hash-1"}
	if err := dbs.WithTx(del.Apply); err != nil {
		t.Fatalf("SessionDeleteOp: %v", err)
	}
	dbs.Read.QueryRow("SELECT COUNT(*) FROM sessions WHERE token_hash='hash-1'").Scan(&n)
	if n != 0 {
		t.Fatalf("删除后 sessions 行数 = %d, want 0", n)
	}
}

func TestLedgerChain(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)
	now := int64(1730000004000)

	first := EarnOp{UUID: "u-c-1", FamilyID: f.familyID, ChildID: f.childID,
		RewardItemID: f.itemID, Delta: 5, BalanceAfter: 5, CreatedAt: now}
	second := EarnOp{UUID: "u-c-2", FamilyID: f.familyID, ChildID: f.childID,
		RewardItemID: f.itemID, Delta: 5, BalanceAfter: 10, CreatedAt: now + 1}
	if err := dbs.WithTx(first.Apply); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := dbs.WithTx(second.Apply); err != nil {
		t.Fatalf("second: %v", err)
	}

	rows, err := dbs.Read.Query(
		"SELECT delta, balance_after FROM ledger WHERE child_id=? ORDER BY id", f.childID)
	if err != nil {
		t.Fatalf("查链: %v", err)
	}
	defer rows.Close()
	want := [][2]int64{{5, 5}, {5, 10}}
	i := 0
	for rows.Next() {
		var d, b int64
		if err := rows.Scan(&d, &b); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if i >= len(want) {
			t.Fatalf("行数超预期")
		}
		if d != want[i][0] || b != want[i][1] {
			t.Errorf("第 %d 行 (delta=%d, bal=%d), want %v", i, d, b, want[i])
		}
		i++
	}
	if i != len(want) {
		t.Errorf("行数 = %d, want %d", i, len(want))
	}
}
