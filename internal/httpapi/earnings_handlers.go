package httpapi

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"good-habit/internal/ledger"
	"good-habit/internal/store"
)

// POST /api/earnings：家长登记得分（热路径——内存原子加 + 异步落盘）。
func (s *server) handleEarning(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		ChildID       int64  `json:"childId"`
		RewardItemID  int64  `json:"rewardItemId"`
		Note          string `json:"note"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	if _, apiErr := s.childOf(sess.FamilyID, body.ChildID); apiErr != nil {
		s.writeAPIErr(w, apiErr)
		return
	}

	// 积分项从内存快照取（active-only、含分值）
	fc := s.deps.Store.Family(sess.FamilyID)
	var item *store.RewardItem
	if fc != nil {
		for i := range fc.Items {
			if fc.Items[i].ID == body.RewardItemID {
				item = &fc.Items[i]
				break
			}
		}
	}
	if item == nil {
		// 区分"不存在"与"已停用"
		var active int64
		err := s.deps.DBs.Read.QueryRow(
			"SELECT active FROM reward_items WHERE id=? AND family_id=?",
			body.RewardItemID, sess.FamilyID,
		).Scan(&active)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "not_found", "积分项不存在")
			return
		}
		if err == nil && active == 0 {
			writeErr(w, http.StatusUnprocessableEntity, "item_inactive", "该积分项已停用")
			return
		}
		writeInternal(w, r, err, "快照缺失积分项")
		return
	}

	note := strings.TrimSpace(body.Note)
	itemTitle := item.Title
	itemID := item.ID
	bal, apiErr := s.applyLedger(sess.FamilyID, body.ChildID, item.Points, func(balanceAfter int64, uuid string) ledger.Op {
		return ledger.EarnOp{
			UUID: uuid, FamilyID: sess.FamilyID, ChildID: body.ChildID,
			RewardItemID: itemID, Delta: item.Points, BalanceAfter: balanceAfter,
			Note: note, CreatedAt: time.Now().UnixMilli(),
		}
	})
	if apiErr != nil {
		s.writeAPIErr(w, apiErr)
		return
	}
	s.deps.Store.BumpEarnsToday(sess.FamilyID, s.dayKey())
	writeJSON(w, http.StatusOK, map[string]any{
		"entry": map[string]any{
			"delta": item.Points, "balanceAfter": bal,
			"kind": "earn", "itemTitle": itemTitle,
		},
	})
}

// POST /api/adjustments：家长手动 ±积分（必须填原因）。
func (s *server) handleAdjustment(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		ChildID int64  `json:"childId"`
		Delta   int64  `json:"delta"`
		Note    string `json:"note"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Delta == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_delta", "调整值不能为 0")
		return
	}
	body.Note = strings.TrimSpace(body.Note)
	if body.Note == "" {
		writeErr(w, http.StatusUnprocessableEntity, "note_required", "请填写调整原因")
		return
	}
	if _, apiErr := s.childOf(sess.FamilyID, body.ChildID); apiErr != nil {
		s.writeAPIErr(w, apiErr)
		return
	}

	note := body.Note
	delta := body.Delta
	bal, apiErr := s.applyLedger(sess.FamilyID, body.ChildID, delta, func(balanceAfter int64, uuid string) ledger.Op {
		return ledger.AdjustOp{
			UUID: uuid, FamilyID: sess.FamilyID, ChildID: body.ChildID,
			Delta: delta, BalanceAfter: balanceAfter,
			Note: note, CreatedAt: time.Now().UnixMilli(),
		}
	})
	if apiErr != nil {
		s.writeAPIErr(w, apiErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entry": map[string]any{
			"delta": delta, "balanceAfter": bal, "kind": "adjust",
		},
	})
}

// applyLedger 是热路径核心：child.mu 内"检查不透支 → 变更 → 入队"原子完成。
// mkOp 拿到入队时的余额快照与 uuid 构造落盘算子。
func (s *server) applyLedger(familyID, childID, delta int64, mkOp func(balanceAfter int64, uuid string) ledger.Op) (int64, *APIError) {
	c := s.deps.Store.Child(childID)
	c.Lock()
	defer c.Unlock()

	if c.Balance+delta < 0 {
		return 0, &APIError{Status: http.StatusConflict, Code: "insufficient_points", Msg: "积分不足"}
	}
	c.Balance += delta
	bal := c.Balance
	uuid := ledger.NewUUID()
	s.deps.Enqueue(mkOp(bal, uuid))
	return bal, nil
}

// childOf 校验孩子存在、属于本家庭且启用。
func (s *server) childOf(familyID, childID int64) (bool, *APIError) {
	var one int64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT 1 FROM users WHERE id=? AND family_id=? AND role='child' AND active=1",
		childID, familyID,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, &APIError{Status: http.StatusNotFound, Code: "not_found", Msg: "孩子不存在"}
	}
	if err != nil {
		return false, &APIError{Status: http.StatusInternalServerError, Code: "internal", Msg: "查孩子失败"}
	}
	return true, nil
}

func (s *server) writeAPIErr(w http.ResponseWriter, e *APIError) {
	if e == nil {
		return
	}
	writeErr(w, e.Status, e.Code, e.Msg)
}

// dayKey 按 Cfg.DayTZ 计算"今天"（overview/计数共用）。
func (s *server) dayKey() string {
	loc := time.Local
	if s.deps.Cfg.DayTZ != "" {
		if l, err := time.LoadLocation(s.deps.Cfg.DayTZ); err == nil {
			loc = l
		} else {
			slog.Warn("DayTZ 无效，退回服务器本地时区", "tz", s.deps.Cfg.DayTZ)
		}
	}
	return time.Now().In(loc).Format("2006-01-02")
}
