package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"good-habit/internal/db"
)

// LoadAll 启动时从 DB 重建全部内存态（设计文档 §6.7）：
//  1. active 孩子余额：users.balance 与账单 SUM 对账，不一致以账单为准（WithTx 修复 + 警告日志）；
//  2. 未过期会话（联 users 过滤已停用账号）；
//  3. 逐家庭配置快照（active 积分项/奖品按 sort + 定价）。
func (s *Store) LoadAll(dbs *db.DBs) error {
	if err := s.loadChildren(dbs); err != nil {
		return err
	}
	if err := s.loadSessions(dbs); err != nil {
		return err
	}

	var famIDs []int64
	rows, err := dbs.Read.Query("SELECT id FROM families ORDER BY id")
	if err != nil {
		return fmt.Errorf("查 families: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan family id: %w", err)
		}
		famIDs = append(famIDs, id)
	}
	rows.Close()

	for _, fid := range famIDs {
		if err := s.loadFamily(dbs, fid); err != nil {
			return err
		}
	}
	return nil
}

// ReloadFamily 家长 CRUD 后重载单个家庭的配置快照。
func (s *Store) ReloadFamily(dbs *db.DBs, familyID int64) error {
	return s.loadFamily(dbs, familyID)
}

func (s *Store) loadChildren(dbs *db.DBs) error {
	rows, err := dbs.Read.Query(
		"SELECT u.id, u.balance, COALESCE(SUM(l.delta), 0) FROM users u LEFT JOIN ledger l ON l.child_id = u.id WHERE u.role='child' AND u.active=1 GROUP BY u.id, u.balance")
	if err != nil {
		return fmt.Errorf("查孩子余额: %w", err)
	}
	type rowT struct {
		id, stated, sum int64
	}
	var rowsData []rowT
	for rows.Next() {
		var r rowT
		if err := rows.Scan(&r.id, &r.stated, &r.sum); err != nil {
			rows.Close()
			return fmt.Errorf("scan 孩子余额: %w", err)
		}
		rowsData = append(rowsData, r)
	}
	rows.Close()

	for _, r := range rowsData {
		if r.stated != r.sum {
			slog.Warn("启动对账：users.balance 与账单 SUM 不一致，以账单为准修复",
				"child_id", r.id, "stated", r.stated, "ledger_sum", r.sum)
			if err := dbs.WithTx(func(tx *sql.Tx) error {
				_, err := tx.Exec("UPDATE users SET balance=? WHERE id=?", r.sum, r.id)
				return err
			}); err != nil {
				return fmt.Errorf("修复孩子 %d 余额: %w", r.id, err)
			}
		}
		c := s.Child(r.id)
		c.mu.Lock()
		c.Balance = r.sum
		c.mu.Unlock()
	}

	// 恢复待选局指针（规则保证每孩子至多一局）
	prows, err := dbs.Read.Query(
		"SELECT uuid, child_id, created_at + 172800000 FROM draws WHERE state='pending_choice'")
	if err != nil {
		return fmt.Errorf("查待选局: %w", err)
	}
	type pendT struct {
		uuid     string
		childID  int64
		deadline int64
	}
	var pends []pendT
	for prows.Next() {
		var p pendT
		if err := prows.Scan(&p.uuid, &p.childID, &p.deadline); err != nil {
			prows.Close()
			return fmt.Errorf("scan 待选局: %w", err)
		}
		pends = append(pends, p)
	}
	prows.Close()
	for _, p := range pends {
		if v, ok := s.children.Load(p.childID); ok {
			c := v.(*ChildAcct)
			c.mu.Lock()
			c.PendingUUID = p.uuid
			c.PendingDeadline = p.deadline
			c.mu.Unlock()
		}
	}

	// 恢复待发放计数
	urows, err := dbs.Read.Query(
		"SELECT child_id, COUNT(*) FROM draws WHERE state='completed' AND fulfilled_at IS NULL GROUP BY child_id")
	if err != nil {
		return fmt.Errorf("查待发放: %w", err)
	}
	type unfulT struct {
		childID int64
		n       int64
	}
	var unfuls []unfulT
	for urows.Next() {
		var u unfulT
		if err := urows.Scan(&u.childID, &u.n); err != nil {
			urows.Close()
			return fmt.Errorf("scan 待发放: %w", err)
		}
		unfuls = append(unfuls, u)
	}
	urows.Close()
	for _, u := range unfuls {
		if v, ok := s.children.Load(u.childID); ok {
			c := v.(*ChildAcct)
			c.mu.Lock()
			c.Unfulfilled = u.n
			c.mu.Unlock()
		}
	}
	return nil
}

func (s *Store) loadSessions(dbs *db.DBs) error {
	now := time.Now().UnixMilli()
	rows, err := dbs.Read.Query(
		`SELECT s.token_hash, s.user_id, u.family_id, u.role, s.expires_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.expires_at > ? AND u.active = 1`, now)
	if err != nil {
		return fmt.Errorf("查会话: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.TokenHash, &sess.UserID, &sess.FamilyID, &sess.Role, &sess.ExpiresAt); err != nil {
			return fmt.Errorf("scan 会话: %w", err)
		}
		s.PutSession(sess)
	}
	return rows.Err()
}

func (s *Store) loadFamily(dbs *db.DBs, familyID int64) error {
	fc := &FamilyConfig{FamilyID: familyID}
	if err := dbs.Read.QueryRow(
		"SELECT single_draw_cost, double_draw_cost FROM families WHERE id=?", familyID,
	).Scan(&fc.SingleCost, &fc.DoubleCost); err != nil {
		return fmt.Errorf("查家庭 %d 定价: %w", familyID, err)
	}

	rows, err := dbs.Read.Query(
		"SELECT id, title, points FROM reward_items WHERE family_id=? AND active=1 ORDER BY sort, id", familyID)
	if err != nil {
		return fmt.Errorf("查家庭 %d 积分项: %w", familyID, err)
	}
	for rows.Next() {
		var it RewardItem
		if err := rows.Scan(&it.ID, &it.Title, &it.Points); err != nil {
			rows.Close()
			return fmt.Errorf("scan 积分项: %w", err)
		}
		fc.Items = append(fc.Items, it)
	}
	rows.Close()

	// 积分项适用孩子（0002）：无关联行 = 对所有孩子生效；有行 = 恰好这些孩子。
	// 按 reward_item_id, child_id 排序，保证每个 ChildIDs 内 child_id 升序。
	rows, err = dbs.Read.Query(
		`SELECT ric.reward_item_id, ric.child_id
		 FROM reward_item_children ric
		 JOIN reward_items ri ON ri.id = ric.reward_item_id
		 WHERE ri.family_id=? AND ri.active=1
		 ORDER BY ric.reward_item_id, ric.child_id`, familyID)
	if err != nil {
		return fmt.Errorf("查家庭 %d 积分项适用孩子: %w", familyID, err)
	}
	childMap := map[int64][]int64{}
	for rows.Next() {
		var itemID, childID int64
		if err := rows.Scan(&itemID, &childID); err != nil {
			rows.Close()
			return fmt.Errorf("scan 积分项适用孩子: %w", err)
		}
		childMap[itemID] = append(childMap[itemID], childID)
	}
	rows.Close()
	for i := range fc.Items {
		fc.Items[i].ChildIDs = childMap[fc.Items[i].ID]
	}

	rows, err = dbs.Read.Query(
		"SELECT id, name, emoji FROM prizes WHERE family_id=? AND active=1 ORDER BY sort, id", familyID)
	if err != nil {
		return fmt.Errorf("查家庭 %d 奖品: %w", familyID, err)
	}
	for rows.Next() {
		var p Prize
		if err := rows.Scan(&p.ID, &p.Name, &p.Emoji); err != nil {
			rows.Close()
			return fmt.Errorf("scan 奖品: %w", err)
		}
		fc.Prizes = append(fc.Prizes, p)
	}
	rows.Close()

	s.SetFamily(fc)
	return nil
}
