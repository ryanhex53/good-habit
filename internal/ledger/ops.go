// Package ledger 定义组提交的落盘算子（设计文档 §6.5）。
// 每个 Op 在调用方事务内原子完成自己的全部行写入；
// 涉及孩子余额的 Op 同时维护 users.balance 冗余列（账单 SUM 为准的冗余）。
package ledger

import (
	"database/sql"
	"fmt"
)

// Op 是可进组提交队列（容量 8192 的 channel）的落盘算子。
type Op interface {
	// Apply 在调用方给出的事务内执行本算子的全部 SQL。
	Apply(tx *sql.Tx) error
}

// EarnOp：登记得分（kind=earn）。UserAgent 为家长设备的原始 UA（空串 = 未记录）。
type EarnOp struct {
	UUID         string
	FamilyID     int64
	ChildID      int64
	RewardItemID int64
	Delta        int64
	BalanceAfter int64 // 入队时在 child.mu 内算好的快照
	Note         string
	UserAgent    string
	CreatedAt    int64 // Unix 毫秒
}

// AdjustOp：家长手动调整（kind=adjust）。
type AdjustOp struct {
	UUID         string
	FamilyID     int64
	ChildID      int64
	Delta        int64
	BalanceAfter int64
	Note         string
	UserAgent    string
	CreatedAt    int64
}

// DrawOp：一次抽奖（kind=draw）——同一事务内先落 draws 行、
// 再以其 id 作 ref_draw_id 落 ledger 行，最后更新 users.balance。
type DrawOp struct {
	UUID     string
	FamilyID int64
	ChildID  int64
	Mode     string // "single" | "double"
	Cost     int64
	// Prize0ID 总有值；Prize1ID 仅双抽（0 表示单抽 → 存 NULL）。
	Prize0ID, Prize1ID int64
	State              string // "pending_choice" | "completed"
	Picked             *int   // nil = 未选
	Delta              int64
	BalanceAfter       int64
	CreatedAt          int64
}

// SessionUpsertOp：会话写入（登录）。
type SessionUpsertOp struct {
	TokenHash  string
	UserID     int64
	ExpiresAt  int64
	CreatedAt  int64
}

// SessionDeleteOp：会话删除（登出）。
type SessionDeleteOp struct {
	TokenHash string
}

func (o EarnOp) Apply(tx *sql.Tx) error {
	// user_agent 列 NOT NULL：直接传字符串（空串即可），不能用 nullString
	_, err := tx.Exec(
		`INSERT INTO ledger (uuid, family_id, child_id, delta, balance_after, kind, ref_reward_item_id, note, user_agent, created_at)
		 VALUES (?,?,?,?,?,'earn',?,?,?,?)`,
		o.UUID, o.FamilyID, o.ChildID, o.Delta, o.BalanceAfter,
		nullInt64(o.RewardItemID), nullString(o.Note), o.UserAgent, o.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("earn 插入 ledger: %w", err)
	}
	return setBalance(tx, o.ChildID, o.BalanceAfter)
}

func (o AdjustOp) Apply(tx *sql.Tx) error {
	_, err := tx.Exec(
		`INSERT INTO ledger (uuid, family_id, child_id, delta, balance_after, kind, note, user_agent, created_at)
		 VALUES (?,?,?,?,?,'adjust',?,?,?)`,
		o.UUID, o.FamilyID, o.ChildID, o.Delta, o.BalanceAfter, nullString(o.Note), o.UserAgent, o.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("adjust 插入 ledger: %w", err)
	}
	return setBalance(tx, o.ChildID, o.BalanceAfter)
}

func (o DrawOp) Apply(tx *sql.Tx) error {
	res, err := tx.Exec(
		`INSERT INTO draws (uuid, family_id, child_id, mode, cost, prize0_id, prize1_id, picked, state, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		o.UUID, o.FamilyID, o.ChildID, o.Mode, o.Cost,
		o.Prize0ID, nullInt64(o.Prize1ID), nullablePicked(o.Picked), o.State, o.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("draw 插入 draws: %w", err)
	}
	drawID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("draw 取 LastInsertId: %w", err)
	}
	_, err = tx.Exec(
		`INSERT INTO ledger (uuid, family_id, child_id, delta, balance_after, kind, ref_draw_id, created_at)
		 VALUES (?,?,?,?,?,'draw',?,?)`,
		o.UUID, o.FamilyID, o.ChildID, o.Delta, o.BalanceAfter, drawID, o.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("draw 插入 ledger: %w", err)
	}
	return setBalance(tx, o.ChildID, o.BalanceAfter)
}

func (o SessionUpsertOp) Apply(tx *sql.Tx) error {
	_, err := tx.Exec(
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?,?,?,?)
		 ON CONFLICT(token_hash) DO UPDATE SET user_id=excluded.user_id, expires_at=excluded.expires_at`,
		o.TokenHash, o.UserID, o.ExpiresAt, o.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

func (o SessionDeleteOp) Apply(tx *sql.Tx) error {
	if _, err := tx.Exec("DELETE FROM sessions WHERE token_hash=?", o.TokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func setBalance(tx *sql.Tx, childID, balance int64) error {
	res, err := tx.Exec("UPDATE users SET balance=? WHERE id=?", balance, childID)
	if err != nil {
		return fmt.Errorf("更新 users.balance: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("更新 users.balance: child %d 影响 %d 行（应恰 1 行）", childID, n)
	}
	return nil
}

func nullInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullablePicked(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
