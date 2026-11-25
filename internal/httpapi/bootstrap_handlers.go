package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"good-habit/internal/seed"
)

var errAlreadyBootstrapped = errors.New("already_bootstrapped")

type seedItem struct {
	Title  string `json:"title"`
	Points int64  `json:"points"`
}

type seedPrize struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

// POST /api/bootstrap：一键导入种子数据（缺省用附录 A）；
// 家庭已有任何积分项或奖品 → 409 already_bootstrapped。
func (s *server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		Items      *[]seedItem  `json:"items"`
		Prizes     *[]seedPrize `json:"prizes"`
		SingleCost int64        `json:"singleCost"`
		DoubleCost int64        `json:"doubleCost"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	defItems, defPrizes := seed.Data()
	items := make([]seedItem, 0, len(defItems))
	if body.Items != nil {
		items = append(items, *body.Items...)
	} else {
		for _, it := range defItems {
			items = append(items, seedItem{Title: it.Title, Points: it.Points})
		}
	}
	prizes := make([]seedPrize, 0, len(defPrizes))
	if body.Prizes != nil {
		prizes = append(prizes, *body.Prizes...)
	} else {
		for _, pz := range defPrizes {
			prizes = append(prizes, seedPrize{Name: pz.Name, Emoji: pz.Emoji})
		}
	}

	for _, it := range items {
		if strings.TrimSpace(it.Title) == "" || it.Points < 1 || it.Points > 100000 {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_items", "积分项标题与分值不合法")
			return
		}
	}
	for _, pz := range prizes {
		if strings.TrimSpace(pz.Name) == "" {
			writeErr(w, http.StatusUnprocessableEntity, "invalid_prizes", "奖品名称不能为空")
			return
		}
	}
	// 定价可选：要么成对提供，要么都不提供
	if (body.SingleCost == 0) != (body.DoubleCost == 0) {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_cost", "singleCost/doubleCost 需成对提供")
		return
	}
	if body.SingleCost != 0 && (body.SingleCost < 1 || body.DoubleCost < body.SingleCost || body.DoubleCost > 999) {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_cost", "需满足 1 ≤ 单抽价 ≤ 双抽价 ≤ 999")
		return
	}

	now := time.Now().UnixMilli()
	err := s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		// 事务内复检，防并发双导入
		var one int64
		err := tx.QueryRow(
			"SELECT 1 FROM reward_items WHERE family_id=? LIMIT 1", sess.FamilyID).Scan(&one)
		if err == nil {
			return errAlreadyBootstrapped
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		err = tx.QueryRow(
			"SELECT 1 FROM prizes WHERE family_id=? LIMIT 1", sess.FamilyID).Scan(&one)
		if err == nil {
			return errAlreadyBootstrapped
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		for i, it := range items {
			if _, err := tx.Exec(
				"INSERT INTO reward_items (family_id, title, points, sort, created_at) VALUES (?,?,?,?,?)",
				sess.FamilyID, strings.TrimSpace(it.Title), it.Points, i, now); err != nil {
				return err
			}
		}
		for i, pz := range prizes {
			emoji := strings.TrimSpace(pz.Emoji)
			if emoji == "" {
				emoji = "🎁"
			}
			if _, err := tx.Exec(
				"INSERT INTO prizes (family_id, name, emoji, sort, created_at) VALUES (?,?,?,?,?)",
				sess.FamilyID, strings.TrimSpace(pz.Name), emoji, i, now); err != nil {
				return err
			}
		}
		if body.SingleCost != 0 {
			if _, err := tx.Exec(
				"UPDATE families SET single_draw_cost=?, double_draw_cost=? WHERE id=?",
				body.SingleCost, body.DoubleCost, sess.FamilyID); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errAlreadyBootstrapped) {
		writeErr(w, http.StatusConflict, "already_bootstrapped", "已导入过，请在管理页调整")
		return
	}
	if err != nil {
		writeInternal(w, r, err, "导入失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	writeJSON(w, http.StatusOK, map[string]any{
		"importedItems": len(items), "importedPrizes": len(prizes),
	})
}
