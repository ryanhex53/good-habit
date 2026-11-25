package httpapi

import (
	"database/sql"
	"net/http"
)

// GET /api/wheel-config：读家庭抽奖定价（DB 读，注册后即可用）。
func (s *server) handleWheelConfigGet(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var single, dbl int64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT single_draw_cost, double_draw_cost FROM families WHERE id=?", sess.FamilyID,
	).Scan(&single, &dbl)
	if err != nil {
		writeInternal(w, r, err, "查定价失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"singleCost": single, "doubleCost": dbl})
}

// PUT /api/wheel-config：改定价，校验 1 ≤ single ≤ double ≤ 999。
func (s *server) handleWheelConfigPut(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		SingleCost int64 `json:"singleCost"`
		DoubleCost int64 `json:"doubleCost"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.SingleCost < 1 || body.DoubleCost < body.SingleCost || body.DoubleCost > 999 {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_cost",
			"需满足 1 ≤ 单抽价 ≤ 双抽价 ≤ 999")
		return
	}
	err := s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			"UPDATE families SET single_draw_cost=?, double_draw_cost=? WHERE id=?",
			body.SingleCost, body.DoubleCost, sess.FamilyID)
		return err
	})
	if err != nil {
		writeInternal(w, r, err, "更新定价失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	writeJSON(w, http.StatusOK, map[string]any{"singleCost": body.SingleCost, "doubleCost": body.DoubleCost})
}
