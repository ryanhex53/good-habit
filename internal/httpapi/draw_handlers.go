package httpapi

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"good-habit/internal/ledger"
	"good-habit/internal/ratelimit"
	"good-habit/internal/store"
)

type drawPrize struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

type drawResp struct {
	Draw struct {
		UUID    string      `json:"uuid"`
		Mode    string      `json:"mode"`
		State   string      `json:"state"`
		Prize   *drawPrize  `json:"prize,omitempty"`
		Choices []drawPrize `json:"choices,omitempty"`
	} `json:"draw"`
	Entry struct {
		Delta        int64  `json:"delta"`
		BalanceAfter int64  `json:"balanceAfter"`
		Kind         string `json:"kind"`
	} `json:"entry"`
}

// drawService 承载抽奖热路径：每孩子限频 + child.mu 内原子扣费/开局。
type drawService struct {
	store        *store.Store
	enqueue      func(ledger.Op)
	childLimiter *ratelimit.Limiter
}

func newDrawService(deps Deps) *drawService {
	return &drawService{
		store:        deps.Store,
		enqueue:      deps.Enqueue,
		childLimiter: ratelimit.New(time.Minute, 30),
	}
}

// Execute 执行一次抽奖（single 完成即中；double 开 pending_choice 局）。
func (ds *drawService) Execute(familyID, childID int64, mode string) (drawResp, *APIError) {
	if mode != "single" && mode != "double" {
		return drawResp{}, &APIError{Status: http.StatusUnprocessableEntity, Code: "bad_mode", Msg: "mode 只能是 single 或 double"}
	}
	if !ds.childLimiter.Allow(strconv.FormatInt(childID, 10)) {
		return drawResp{}, &APIError{Status: http.StatusTooManyRequests, Code: "rate_limited", Msg: "抽太快啦，歇一口气"}
	}

	fc := ds.store.Family(familyID)
	if fc == nil || len(fc.Prizes) == 0 {
		return drawResp{}, &APIError{Status: http.StatusConflict, Code: "no_active_prizes", Msg: "转盘还没有奖品"}
	}
	cost := fc.DoubleCost
	if mode == "single" {
		cost = fc.SingleCost
	}

	c := ds.store.Child(childID)
	c.Lock()
	defer c.Unlock()
	if c.PendingUUID != "" {
		return drawResp{}, &APIError{Status: http.StatusConflict, Code: "pending_choice_exists", Msg: "先选完上一局的奖品"}
	}
	if c.Balance < cost {
		return drawResp{}, &APIError{Status: http.StatusConflict, Code: "insufficient_points", Msg: "积分不够啦"}
	}

	p0 := pickPrize(fc.Prizes)
	var p1 store.Prize
	prize1ID := int64(0)
	if mode == "double" {
		p1 = pickPrize(fc.Prizes)
		prize1ID = p1.ID
	}

	c.Balance -= cost
	bal := c.Balance
	uuid := ledger.NewUUID()
	now := time.Now().UnixMilli()

	state := "completed"
	var picked *int
	if mode == "single" {
		z := 0
		picked = &z
		c.Unfulfilled++
	} else {
		state = "pending_choice"
		c.PendingUUID = uuid
		c.PendingDeadline = now + 48*3600*1000
	}

	ds.enqueue(ledger.DrawOp{
		UUID: uuid, FamilyID: familyID, ChildID: childID,
		Mode: mode, Cost: cost, Prize0ID: p0.ID, Prize1ID: prize1ID,
		State: state, Picked: picked,
		Delta: -cost, BalanceAfter: bal, CreatedAt: now,
	})

	var resp drawResp
	resp.Draw.UUID = uuid
	resp.Draw.Mode = mode
	resp.Draw.State = state
	if mode == "single" {
		pv := toPrizeView(p0)
		resp.Draw.Prize = &pv
	} else {
		resp.Draw.Choices = []drawPrize{toPrizeView(p0), toPrizeView(p1)}
	}
	resp.Entry.Delta = -cost
	resp.Entry.BalanceAfter = bal
	resp.Entry.Kind = "draw"
	return resp, nil
}

func toPrizeView(p store.Prize) drawPrize {
	return drawPrize{ID: p.ID, Name: p.Name, Emoji: p.Emoji}
}

// pickPrize 等概率抽取（crypto/rand 拒绝采样天然均匀）。
func pickPrize(prizes []store.Prize) store.Prize {
	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(prizes))))
	if err != nil {
		panic("draw: crypto/rand 失败: " + err.Error())
	}
	return prizes[i.Int64()]
}

// POST /api/draws（child）：发起抽奖。
func (s *server) handleDraw(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		Mode string `json:"mode"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	resp, apiErr := s.draw.Execute(sess.FamilyID, sess.UserID, body.Mode)
	if apiErr != nil {
		s.writeAPIErr(w, apiErr)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /api/draws/{uuid}/choose（child）：双抽二选一。
func (s *server) handleChoose(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	uuid := r.PathValue("uuid")
	var body struct {
		Pick int64 `json:"pick"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Pick != 0 && body.Pick != 1 {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_pick", "pick 只能是 0 或 1")
		return
	}

	var drawID, childID, prize0, prize1 int64
	var state string
	err := s.deps.DBs.Read.QueryRow(
		"SELECT id, child_id, state, prize0_id, prize1_id FROM draws WHERE uuid=? AND family_id=?",
		uuid, sess.FamilyID,
	).Scan(&drawID, &childID, &state, &prize0, &prize1)
	if errors.Is(err, sql.ErrNoRows) && s.deps.Flush != nil {
		// 异步组提交窗口内行可能未落盘：先屏障落盘再重查一次
		if ferr := s.deps.Flush(); ferr == nil {
			err = s.deps.DBs.Read.QueryRow(
				"SELECT id, child_id, state, prize0_id, prize1_id FROM draws WHERE uuid=? AND family_id=?",
				uuid, sess.FamilyID,
			).Scan(&drawID, &childID, &state, &prize0, &prize1)
		}
	}
	if errors.Is(err, sql.ErrNoRows) || (err == nil && childID != sess.UserID) {
		// 不存在与"不是你的局"统一 404，不泄露他人抽奖存在性
		writeErr(w, http.StatusNotFound, "not_found", "抽奖不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "查抽奖失败")
		return
	}
	if state != "pending_choice" {
		writeErr(w, http.StatusConflict, "draw_completed", "这一局已经选过了")
		return
	}

	updated := false
	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"UPDATE draws SET picked=?, state='completed' WHERE id=? AND state='pending_choice'",
			body.Pick, drawID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		updated = n == 1
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "选择失败")
		return
	}
	if !updated {
		// 并发（如清扫）先完成
		writeErr(w, http.StatusConflict, "draw_completed", "这一局已经选过了")
		return
	}

	c := s.deps.Store.Child(childID)
	c.Lock()
	if c.PendingUUID == uuid {
		c.PendingUUID = ""
		c.PendingDeadline = 0
	}
	c.Unfulfilled++
	c.Unlock()

	chosenID := prize0
	if body.Pick == 1 {
		chosenID = prize1
	}
	var pv drawPrize
	if err := s.deps.DBs.Read.QueryRow(
		"SELECT id, name, emoji FROM prizes WHERE id=?", chosenID,
	).Scan(&pv.ID, &pv.Name, &pv.Emoji); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "查奖品失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"draw": map[string]any{
			"uuid": uuid, "prize": pv, "state": "completed",
		},
	})
}

// PATCH /api/draws/{uuid}/fulfill（parent）：标记奖品已发放。
func (s *server) handleFulfill(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	uuid := r.PathValue("uuid")

	var drawID, childID int64
	var state string
	var fulfilled sql.NullInt64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT id, child_id, state, fulfilled_at FROM draws WHERE uuid=? AND family_id=?",
		uuid, sess.FamilyID,
	).Scan(&drawID, &childID, &state, &fulfilled)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "not_found", "抽奖不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "查抽奖失败")
		return
	}
	if fulfilled.Valid {
		writeErr(w, http.StatusConflict, "already_fulfilled", "已经发放过了")
		return
	}
	if state != "completed" {
		writeErr(w, http.StatusConflict, "not_completed", "孩子还没选完奖品，选完才能发放")
		return
	}

	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"UPDATE draws SET fulfilled_at=? WHERE id=? AND fulfilled_at IS NULL",
			time.Now().UnixMilli(), drawID)
		return err
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "发放失败")
		return
	}

	c := s.deps.Store.Child(childID)
	c.Lock()
	if c.Unfulfilled > 0 {
		c.Unfulfilled--
	}
	c.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"uuid": uuid, "ok": true})
}
