package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
)

type guideItem struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Points int64  `json:"points"`
}

// GET /api/guide（child）：得分指南——只读快照（做什么能得分），只含适用于该孩子的项。
func (s *server) handleGuide(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	items := []guideItem{}
	if fc := s.deps.Store.Family(sess.FamilyID); fc != nil {
		for _, it := range fc.Items {
			if !it.AppliesTo(sess.UserID) {
				continue
			}
			items = append(items, guideItem{ID: it.ID, Title: it.Title, Points: it.Points})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// GET /api/wheel（child）：转盘快照 + 余额 + 待选局。
func (s *server) handleWheel(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	fc := s.deps.Store.Family(sess.FamilyID)
	if fc == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"prizes": []any{}, "singleCost": 5, "doubleCost": 7,
			"balance": 0, "pending": nil,
		})
		return
	}

	c := s.deps.Store.Child(sess.UserID)
	c.Lock()
	bal := c.Balance
	pendingUUID := c.PendingUUID
	deadline := c.PendingDeadline
	c.Unlock()

	pending := any(nil)
	if pendingUUID != "" {
		// choices 行可能仍在组提交窗口内：Flush 后重查
		var p0, p1 struct {
			ID    int64
			Name  string
			Emoji string
		}
		q := `SELECT d.prize0_id, p0.name, p0.emoji, COALESCE(d.prize1_id, d.prize0_id), COALESCE(p1.name, p0.name), COALESCE(p1.emoji, p0.emoji)
			FROM draws d
			JOIN prizes p0 ON p0.id = d.prize0_id
			LEFT JOIN prizes p1 ON p1.id = d.prize1_id
			WHERE d.uuid=? AND d.family_id=?`
		err := s.deps.DBs.Read.QueryRow(q, pendingUUID, sess.FamilyID).
			Scan(&p0.ID, &p0.Name, &p0.Emoji, &p1.ID, &p1.Name, &p1.Emoji)
		if errors.Is(err, sql.ErrNoRows) && s.deps.Flush != nil {
			if ferr := s.deps.Flush(); ferr == nil {
				err = s.deps.DBs.Read.QueryRow(q, pendingUUID, sess.FamilyID).
					Scan(&p0.ID, &p0.Name, &p0.Emoji, &p1.ID, &p1.Name, &p1.Emoji)
			}
		}
		if err == nil {
			pending = map[string]any{
				"uuid":     pendingUUID,
				"deadline": deadline,
				"choices": []drawPrize{
					{ID: p0.ID, Name: p0.Name, Emoji: p0.Emoji},
					{ID: p1.ID, Name: p1.Name, Emoji: p1.Emoji},
				},
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			writeInternal(w, r, err, "查待选局失败")
			return
		}
	}

	prizes := []drawPrize{}
	for _, p := range fc.Prizes {
		prizes = append(prizes, drawPrize{ID: p.ID, Name: p.Name, Emoji: p.Emoji})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"prizes":     prizes,
		"singleCost": fc.SingleCost,
		"doubleCost": fc.DoubleCost,
		"balance":    bal,
		"pending":    pending,
	})
}

// GET /api/overview（parent）：各孩子余额/待发放/待选 + 今日得分笔数（内存直读）。
func (s *server) handleOverview(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	rows, err := s.deps.DBs.Read.Query(
		"SELECT id, display_name FROM users WHERE family_id=? AND role='child' AND active=1 ORDER BY id",
		sess.FamilyID)
	if err != nil {
		writeInternal(w, r, err, "查孩子失败")
		return
	}
	type ovChild struct {
		ID          int64  `json:"id"`
		DisplayName string `json:"displayName"`
		Balance     int64  `json:"balance"`
		Unfulfilled int64  `json:"unfulfilled"`
		HasPending  bool   `json:"hasPending"`
	}
	children := []ovChild{}
	for rows.Next() {
		var c ovChild
		if err := rows.Scan(&c.ID, &c.DisplayName); err != nil {
			rows.Close()
			writeInternal(w, r, err, "读孩子失败")
			return
		}
		bal, pending, unful := s.deps.Store.Child(c.ID).Stats()
		c.Balance, c.Unfulfilled, c.HasPending = bal, unful, pending != ""
		children = append(children, c)
	}
	rows.Close()
	writeJSON(w, http.StatusOK, map[string]any{
		"children":   children,
		"todayEarns": s.deps.Store.EarnsToday(sess.FamilyID, s.dayKey()),
	})
}

type ledgerRow struct {
	ID           int64  `json:"id"`
	CreatedAt    int64  `json:"createdAt"`
	Kind         string `json:"kind"`
	Delta        int64  `json:"delta"`
	BalanceAfter int64  `json:"balanceAfter"`
	ItemTitle    string `json:"itemTitle"`
	PrizeName    string `json:"prizeName"`
	Mode         string `json:"mode"`
	Note         string `json:"note"`
}

// GET /api/ledger：积分账单（child 自动限定自己；parent 可 ?childId= 查全家）。
// 游标分页：?before=<id>&limit=30（≤100），nextBefore 为下一页游标（尾页 null）。
// 行可能滞后一个组提交窗口（≤15ms 量级），设计文档 §4.9 已声明。
func (s *server) handleLedger(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFrom(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated", "请先登录")
		return
	}

	childID := int64(0)
	if sess.Role == "child" {
		childID = sess.UserID
	} else if q := r.URL.Query().Get("childId"); q != "" {
		v, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "childId 不合法")
			return
		}
		childID = v
	}
	before := int64(0)
	if q := r.URL.Query().Get("before"); q != "" {
		v, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "before 不合法")
			return
		}
		before = v
	}
	limit := 30
	if q := r.URL.Query().Get("limit"); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || v < 1 {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "limit 不合法")
			return
		}
		if v > 100 {
			v = 100
		}
		limit = v
	}

	q := `SELECT l.id, l.created_at, l.kind, l.delta, l.balance_after, COALESCE(l.note, ''),
	       COALESCE(ri.title, ''), COALESCE(d.mode, ''), COALESCE(p.name, '')
	FROM ledger l
	LEFT JOIN reward_items ri ON ri.id = l.ref_reward_item_id
	LEFT JOIN draws d         ON d.id  = l.ref_draw_id
	LEFT JOIN prizes p        ON p.id = CASE WHEN d.mode='single' THEN d.prize0_id
	                                         WHEN d.picked=1    THEN d.prize1_id
	                                         ELSE d.prize0_id END
	WHERE l.family_id = ? AND (? = 0 OR l.child_id = ?) AND (? = 0 OR l.id < ?)
	ORDER BY l.id DESC LIMIT ?`
	rows, err := s.deps.DBs.Read.Query(q, sess.FamilyID, childID, childID, before, before, limit+1)
	if err != nil {
		writeInternal(w, r, err, "查账单失败")
		return
	}
	defer rows.Close()
	list := []ledgerRow{}
	for rows.Next() {
		var v ledgerRow
		if err := rows.Scan(&v.ID, &v.CreatedAt, &v.Kind, &v.Delta, &v.BalanceAfter,
			&v.Note, &v.ItemTitle, &v.Mode, &v.PrizeName); err != nil {
			writeInternal(w, r, err, "读账单失败")
			return
		}
		list = append(list, v)
	}

	nextBefore := any(nil)
	if len(list) > limit {
		list = list[:limit]
		nextBefore = list[len(list)-1].ID
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": list, "nextBefore": nextBefore})
}

// GET /api/draws?state=unfulfilled&childId=（parent）：待发放/全部抽奖列表。
func (s *server) handleDrawsList(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	childID := int64(0)
	if q := r.URL.Query().Get("childId"); q != "" {
		v, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "childId 不合法")
			return
		}
		childID = v
	}
	unfulfilled := r.URL.Query().Get("state") == "unfulfilled"

	q := `SELECT d.uuid, u.display_name, d.mode, d.cost, d.state,
	          COALESCE(d.fulfilled_at, 0), d.created_at, COALESCE(p.name, '')
	FROM draws d
	JOIN users u ON u.id = d.child_id
	LEFT JOIN prizes p ON p.id = CASE WHEN d.picked=1 THEN d.prize1_id ELSE d.prize0_id END
	WHERE d.family_id = ? AND (? = 0 OR d.child_id = ?)`
	args := []any{sess.FamilyID, childID, childID}
	if unfulfilled {
		q += " AND d.state='completed' AND d.fulfilled_at IS NULL"
	}
	q += " ORDER BY d.id DESC LIMIT 200"

	rows, err := s.deps.DBs.Read.Query(q, args...)
	if err != nil {
		writeInternal(w, r, err, "查抽奖失败")
		return
	}
	defer rows.Close()
	type drawListItem struct {
		UUID        string `json:"uuid"`
		ChildName   string `json:"childName"`
		Mode        string `json:"mode"`
		Cost        int64  `json:"cost"`
		PrizeName   string `json:"prizeName"`
		State       string `json:"state"`
		FulfilledAt int64  `json:"fulfilledAt"`
		CreatedAt   int64  `json:"createdAt"`
	}
	list := []drawListItem{}
	for rows.Next() {
		var v drawListItem
		if err := rows.Scan(&v.UUID, &v.ChildName, &v.Mode, &v.Cost, &v.State,
			&v.FulfilledAt, &v.CreatedAt, &v.PrizeName); err != nil {
			writeInternal(w, r, err, "读抽奖失败")
			return
		}
		list = append(list, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"draws": list})
}
