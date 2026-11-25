// Package sweep 清扫超时的 pending_choice 抽奖：
// 超过 48 小时未选择，自动保留 A（picked=0）并完成该局。
// 启动时与每小时各跑一次（app 层接线）。
package sweep

import (
	"database/sql"
	"fmt"
	"time"

	"good-habit/internal/db"
	"good-habit/internal/store"
)

// Run 执行一轮清扫，返回本轮自动完成的局数。
func Run(dbs *db.DBs, st *store.Store) (int, error) {
	cutoff := time.Now().Add(-48 * time.Hour).UnixMilli()
	rows, err := dbs.Read.Query(
		"SELECT uuid, child_id FROM draws WHERE state='pending_choice' AND created_at < ?",
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("查超时局: %w", err)
	}
	type pendingT struct {
		uuid    string
		childID int64
	}
	var list []pendingT
	for rows.Next() {
		var p pendingT
		if err := rows.Scan(&p.uuid, &p.childID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan 超时局: %w", err)
		}
		list = append(list, p)
	}
	rows.Close()

	n := 0
	for _, p := range list {
		updated := false
		err := dbs.WithTx(func(tx *sql.Tx) error {
			res, err := tx.Exec(
				"UPDATE draws SET picked=0, state='completed' WHERE uuid=? AND state='pending_choice'",
				p.uuid)
			if err != nil {
				return err
			}
			c, _ := res.RowsAffected()
			updated = c == 1
			return nil
		})
		if err != nil {
			return n, fmt.Errorf("清扫 %s: %w", p.uuid, err)
		}
		if !updated {
			continue // 已被并发 choose 完成，跳过
		}
		c := st.Child(p.childID)
		c.Lock()
		if c.PendingUUID == p.uuid {
			c.PendingUUID = ""
			c.PendingDeadline = 0
		}
		c.Unfulfilled++
		c.Unlock()
		n++
	}
	return n, nil
}
