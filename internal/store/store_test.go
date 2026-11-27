package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

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

type tf struct {
	aFam, bFam, cFam                int64
	aParent, aChild, bChild, cChild int64
}

func seed(t *testing.T, dbs *db.DBs) tf {
	t.Helper()
	now := time.Now().UnixMilli()
	var f tf
	err := dbs.WithTx(func(tx *sql.Tx) error {
		mkFamily := func(name string, single, dbl int64) (int64, error) {
			res, err := tx.Exec(
				"INSERT INTO families (name, single_draw_cost, double_draw_cost, created_at) VALUES (?,?,?,?)",
				name, single, dbl, now)
			if err != nil {
				return 0, err
			}
			return res.LastInsertId()
		}
		mkUser := func(fid int64, role, uname, disp string, balance int64) (int64, error) {
			res, err := tx.Exec(
				"INSERT INTO users (family_id, role, username, password_hash, display_name, balance, created_at) VALUES (?,?,?,?,?,?,?)",
				fid, role, uname, "h", disp, balance, now)
			if err != nil {
				return 0, err
			}
			return res.LastInsertId()
		}
		var err error
		if f.aFam, err = mkFamily("A家", 6, 9); err != nil {
			return err
		}
		if f.bFam, err = mkFamily("B家", 5, 7); err != nil {
			return err
		}
		if f.cFam, err = mkFamily("C家", 5, 7); err != nil {
			return err
		}
		if f.aParent, err = mkUser(f.aFam, "parent", "pa_1", "爸", 0); err != nil {
			return err
		}
		if f.aChild, err = mkUser(f.aFam, "child", "kid_a", "娃A", 0); err != nil {
			return err
		}
		if f.bChild, err = mkUser(f.bFam, "child", "kid_b", "娃B", 0); err != nil {
			return err
		}
		// C 家孩子：users.balance 故意写错（100），账单 SUM 只有 90
		if f.cChild, err = mkUser(f.cFam, "child", "kid_c", "娃C", 100); err != nil {
			return err
		}
		for _, l := range []struct {
			fid, cid, delta, after int64
		}{
			{f.aFam, f.aChild, 5, 5}, {f.aFam, f.aChild, 5, 10},
			{f.bFam, f.bChild, 3, 3},
			{f.cFam, f.cChild, 90, 90},
		} {
			if _, err := tx.Exec(
				"INSERT INTO ledger (uuid, family_id, child_id, delta, balance_after, kind, created_at) VALUES (?,?,?,?,?,'earn',?)",
				fmt.Sprintf("u-%d-%d", l.fid, l.after), l.fid, l.cid, l.delta, l.after, now); err != nil {
				return err
			}
		}
		// 会话：一条有效、一条已过期
		if _, err := tx.Exec(
			"INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)",
			"hash-valid", f.aParent, now+3600000, now); err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)",
			"hash-expired", f.aChild, now-1000, now); err != nil {
			return err
		}
		// A 家积分项：2 活 + 1 停用（sort 故意逆序，验证按 sort 输出）
		for _, it := range []struct {
			title  string
			points int64
			active int64
			sort   int64
		}{{"X1", 7, 1, 2}, {"X2", 5, 1, 1}, {"X3", 10, 0, 0}} {
			res, err := tx.Exec(
				"INSERT INTO reward_items (family_id, title, points, active, sort, created_at) VALUES (?,?,?,?,?,?)",
				f.aFam, it.title, it.points, it.active, it.sort, now)
			if err != nil {
				return err
			}
			// X2 只给娃A；其余不写关联行 = 所有孩子
			if it.title == "X2" {
				id, _ := res.LastInsertId()
				if _, err := tx.Exec("INSERT INTO reward_item_children (reward_item_id, child_id) VALUES (?,?)", id, f.aChild); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(
			"INSERT INTO reward_items (family_id, title, points, created_at) VALUES (?,?,?,?)",
			f.bFam, "B项", 4, now); err != nil {
			return err
		}
		// A 家奖品：2 活 + 1 停用
		for _, pz := range []struct {
			name   string
			active int64
		}{{"PA", 1}, {"PB", 1}, {"PC", 0}} {
			if _, err := tx.Exec(
				"INSERT INTO prizes (family_id, name, active, created_at) VALUES (?,?,?,?)",
				f.aFam, pz.name, pz.active, now); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(
			"INSERT INTO prizes (family_id, name, created_at) VALUES (?,?,?)",
			f.bFam, "PB奖", now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return f
}

func TestLoadAllRebuild(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)

	s := New()
	if err := s.LoadAll(dbs); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if got := s.Child(f.aChild).Balance; got != 10 {
		t.Errorf("A 孩子余额 = %d, want 10", got)
	}
	if got := s.Child(f.bChild).Balance; got != 3 {
		t.Errorf("B 孩子余额 = %d, want 3", got)
	}

	sess, ok := s.GetSession("hash-valid")
	if !ok {
		t.Fatal("有效会话应加载")
	}
	if sess.UserID != f.aParent || sess.FamilyID != f.aFam || sess.Role != "parent" {
		t.Errorf("会话内容不符: %+v", sess)
	}
	if _, ok := s.GetSession("hash-expired"); ok {
		t.Error("过期会话不应加载")
	}

	fc := s.Family(f.aFam)
	if fc == nil {
		t.Fatal("A 家配置应已加载")
	}
	if len(fc.Items) != 2 {
		t.Fatalf("A 家 active 积分项 = %d, want 2", len(fc.Items))
	}
	if fc.Items[0].Title != "X2" || fc.Items[1].Title != "X1" {
		t.Errorf("积分项应按 sort 排序: %q %q", fc.Items[0].Title, fc.Items[1].Title)
	}
	if fc.Items[1].Points != 7 {
		t.Errorf("X1 分值 = %d, want 7", fc.Items[1].Points)
	}
	if len(fc.Prizes) != 2 {
		t.Errorf("A 家 active 奖品 = %d, want 2", len(fc.Prizes))
	}
	if fc.SingleCost != 6 || fc.DoubleCost != 9 {
		t.Errorf("A 家定价 = %d/%d, want 6/9", fc.SingleCost, fc.DoubleCost)
	}

	fb := s.Family(f.bFam)
	if fb == nil || len(fb.Items) != 1 || len(fb.Prizes) != 1 {
		t.Errorf("B 家配置不符: %+v", fb)
	}

	if len(fc.Items[0].ChildIDs) != 1 || fc.Items[0].ChildIDs[0] != f.aChild {
		t.Errorf("X2 的 ChildIDs = %v, want [%d]", fc.Items[0].ChildIDs, f.aChild)
	}
	if len(fc.Items[1].ChildIDs) != 0 {
		t.Errorf("X1 应为所有孩子（ChildIDs 为空）: %v", fc.Items[1].ChildIDs)
	}
}

func TestRewardItemAppliesTo(t *testing.T) {
	all := RewardItem{ID: 1}
	if !all.AppliesTo(42) || !all.AppliesTo(7) {
		t.Error("未指定孩子应对所有人生效")
	}
	scoped := RewardItem{ID: 2, ChildIDs: []int64{7, 9}}
	if !scoped.AppliesTo(7) || !scoped.AppliesTo(9) {
		t.Error("指定孩子应生效")
	}
	if scoped.AppliesTo(8) {
		t.Error("未指定的孩子不应生效")
	}
}

func TestLoadAllRepairsBalance(t *testing.T) {
	dbs := newDB(t)
	f := seed(t, dbs)

	s := New()
	if err := s.LoadAll(dbs); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if got := s.Child(f.cChild).Balance; got != 90 {
		t.Errorf("C 孩子内存余额 = %d, want 90（账单 SUM 为准）", got)
	}
	var dbBal int64
	if err := dbs.Read.QueryRow("SELECT balance FROM users WHERE id=?", f.cChild).Scan(&dbBal); err != nil {
		t.Fatalf("查 DB: %v", err)
	}
	if dbBal != 90 {
		t.Errorf("DB users.balance = %d, want 90（应已修复）", dbBal)
	}
}

func TestDayCounter(t *testing.T) {
	s := New()
	s.BumpEarnsToday(1, "2026-11-25")
	s.BumpEarnsToday(1, "2026-11-25")
	s.BumpEarnsToday(1, "2026-11-25")
	s.BumpEarnsToday(2, "2026-11-25")

	if got := s.EarnsToday(1, "2026-11-25"); got != 3 {
		t.Errorf("家庭1 今日 = %d, want 3", got)
	}
	if got := s.EarnsToday(2, "2026-11-25"); got != 1 {
		t.Errorf("家庭2 今日 = %d, want 1", got)
	}
	if got := s.EarnsToday(1, "2026-11-26"); got != 0 {
		t.Errorf("家庭1 次日 = %d, want 0", got)
	}
}

func TestSessionMap(t *testing.T) {
	s := New()
	sess := Session{TokenHash: "h", UserID: 7, FamilyID: 3, Role: "child", ExpiresAt: 99}
	s.PutSession(sess)

	got, ok := s.GetSession("h")
	if !ok || got.UserID != 7 || got.Role != "child" {
		t.Errorf("GetSession 不符: %+v ok=%v", got, ok)
	}

	s.DeleteSession("h")
	if _, ok := s.GetSession("h"); ok {
		t.Error("删除后不应存在")
	}
}
