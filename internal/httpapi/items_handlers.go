package httpapi

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type itemView struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Points   int64   `json:"points"`
	Active   bool    `json:"active"`
	Sort     int64   `json:"sort"`
	ChildIDs []int64 `json:"childIds"` // 空 = 所有孩子；响应保证非 nil
}

// GET /api/reward-items：管理视图（含停用项与适用孩子 childIds）。
func (s *server) handleItemsList(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	rows, err := s.deps.DBs.Read.Query(
		"SELECT id, title, points, active, sort FROM reward_items WHERE family_id=? ORDER BY sort, id",
		sess.FamilyID)
	if err != nil {
		writeInternal(w, r, err, "查积分项失败")
		return
	}
	defer rows.Close()
	items := []itemView{}
	for rows.Next() {
		var v itemView
		var active int64
		if err := rows.Scan(&v.ID, &v.Title, &v.Points, &active, &v.Sort); err != nil {
			writeInternal(w, r, err, "读积分项失败")
			return
		}
		v.Active = active == 1
		items = append(items, v)
	}
	ids := make([]int64, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	m, err := s.childIDsByItem(ids)
	if err != nil {
		writeInternal(w, r, err, "查积分项适用孩子失败")
		return
	}
	for i := range items {
		items[i].ChildIDs = m[items[i].ID]
		if items[i].ChildIDs == nil {
			items[i].ChildIDs = []int64{}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/reward-items：新增积分项（可选 childIds 指定适用孩子，缺省所有孩子）。
func (s *server) handleItemCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		Title    string  `json:"title"`
		Points   int64   `json:"points"`
		Sort     int64   `json:"sort"`
		ChildIDs []int64 `json:"childIds"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_title", "请填写标题")
		return
	}
	if body.Points <= 0 || body.Points > 100000 {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_points", "分值须为 1~100000 的正整数")
		return
	}
	childIDs, apiErr := s.resolveChildIDs(sess.FamilyID, body.ChildIDs)
	if apiErr != nil {
		s.writeAPIErr(w, apiErr)
		return
	}

	var id int64
	err := s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"INSERT INTO reward_items (family_id, title, points, sort, created_at) VALUES (?,?,?,?,?)",
			sess.FamilyID, body.Title, body.Points, body.Sort, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		return setItemChildIDs(tx, id, childIDs)
	})
	if err != nil {
		writeInternal(w, r, err, "新建积分项失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writeItemView(w, r, id)
}

// PATCH /api/reward-items/{id}：改名/改分值/排序/启停/适用孩子。
// childIds 用指针区分"没传"（不动范围）与"传了 []"（改为所有孩子）。
func (s *server) handleItemUpdate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "ID 不合法")
		return
	}
	var body struct {
		Title    *string  `json:"title"`
		Points   int64    `json:"points"`
		Active   *bool    `json:"active"`
		Sort     *int64   `json:"sort"`
		ChildIDs *[]int64 `json:"childIds"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Title != nil && strings.TrimSpace(*body.Title) == "" {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_title", "标题不能为空")
		return
	}
	if body.Points != 0 && (body.Points < 1 || body.Points > 100000) {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_points", "分值须为 1~100000 的正整数")
		return
	}
	if !s.owned(w, r, "reward_items", id, sess.FamilyID) {
		return
	}
	// 跨家庭先 404（owned 之上），再校验适用孩子
	var childIDs []int64
	if body.ChildIDs != nil {
		var apiErr *APIError
		childIDs, apiErr = s.resolveChildIDs(sess.FamilyID, *body.ChildIDs)
		if apiErr != nil {
			s.writeAPIErr(w, apiErr)
			return
		}
	}
	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		if body.Title != nil {
			if _, err := tx.Exec("UPDATE reward_items SET title=? WHERE id=?", strings.TrimSpace(*body.Title), id); err != nil {
				return err
			}
		}
		if body.Points != 0 {
			if _, err := tx.Exec("UPDATE reward_items SET points=? WHERE id=?", body.Points, id); err != nil {
				return err
			}
		}
		if body.Sort != nil {
			if _, err := tx.Exec("UPDATE reward_items SET sort=? WHERE id=?", *body.Sort, id); err != nil {
				return err
			}
		}
		if body.Active != nil {
			v := int64(0)
			if *body.Active {
				v = 1
			}
			if _, err := tx.Exec("UPDATE reward_items SET active=? WHERE id=?", v, id); err != nil {
				return err
			}
		}
		if body.ChildIDs != nil {
			if err := setItemChildIDs(tx, id, childIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeInternal(w, r, err, "更新积分项失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writeItemView(w, r, id)
}

// DELETE /api/reward-items/{id}：软删除（active=0）。
func (s *server) handleItemDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "ID 不合法")
		return
	}
	if !s.owned(w, r, "reward_items", id, sess.FamilyID) {
		return
	}
	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE reward_items SET active=0 WHERE id=?", id)
		return err
	})
	if err != nil {
		writeInternal(w, r, err, "删除积分项失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writeItemView(w, r, id)
}

// writeItemView 回读单个积分项（含适用孩子 childIds）并输出。
func (s *server) writeItemView(w http.ResponseWriter, r *http.Request, id int64) {
	var v itemView
	var active int64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT id, title, points, active, sort FROM reward_items WHERE id=?", id,
	).Scan(&v.ID, &v.Title, &v.Points, &active, &v.Sort)
	if err != nil {
		writeInternal(w, r, err, "回读积分项失败")
		return
	}
	v.Active = active == 1
	m, err := s.childIDsByItem([]int64{id})
	if err != nil {
		writeInternal(w, r, err, "查积分项适用孩子失败")
		return
	}
	v.ChildIDs = m[id]
	if v.ChildIDs == nil {
		v.ChildIDs = []int64{}
	}
	writeJSON(w, http.StatusOK, v)
}

// resolveChildIDs 去重并校验适用孩子：nil/空 → 非-nil 空切片（= 所有孩子）；
// 含不存在/非本家庭/非孩子/已停用的 id → 422 child_not_found（与 childOf 同口径）。
func (s *server) resolveChildIDs(familyID int64, ids []int64) ([]int64, *APIError) {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		var one int64
		err := s.deps.DBs.Read.QueryRow(
			"SELECT 1 FROM users WHERE id=? AND family_id=? AND role='child' AND active=1",
			id, familyID,
		).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &APIError{Status: http.StatusUnprocessableEntity, Code: "child_not_found", Msg: "孩子不存在或已停用"}
		}
		if err != nil {
			slog.Error("查孩子失败", "err", err, "family_id", familyID)
			return nil, &APIError{Status: http.StatusInternalServerError, Code: "internal", Msg: "查孩子失败"}
		}
		out = append(out, id)
	}
	return out, nil
}

// childIDsByItem 一次查询一批积分项的适用孩子（itemID → 有序 childID 列表）；
// 入参为空直接返回空 map，不查库。
func (s *server) childIDsByItem(itemIDs []int64) (map[int64][]int64, error) {
	m := make(map[int64][]int64, len(itemIDs))
	if len(itemIDs) == 0 {
		return m, nil
	}
	placeholders := strings.Repeat("?,", len(itemIDs)-1) + "?"
	args := make([]any, len(itemIDs))
	for i, id := range itemIDs {
		args[i] = id
	}
	rows, err := s.deps.DBs.Read.Query(
		"SELECT reward_item_id, child_id FROM reward_item_children WHERE reward_item_id IN ("+placeholders+") ORDER BY reward_item_id, child_id",
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID, childID int64
		if err := rows.Scan(&itemID, &childID); err != nil {
			return nil, err
		}
		m[itemID] = append(m[itemID], childID)
	}
	return m, rows.Err()
}

// setItemChildIDs 事务内重建积分项的适用孩子（先删后插，空 = 删光即所有孩子）。
func setItemChildIDs(tx *sql.Tx, itemID int64, ids []int64) error {
	if _, err := tx.Exec("DELETE FROM reward_item_children WHERE reward_item_id=?", itemID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec(
			"INSERT INTO reward_item_children (reward_item_id, child_id) VALUES (?,?)", itemID, id); err != nil {
			return err
		}
	}
	return nil
}

// owned 校验资源属于本家庭；不属 → 404 并返回 false。
func (s *server) owned(w http.ResponseWriter, r *http.Request, table string, id, familyID int64) bool {
	// table 由代码内常量传入，不接用户输入，无注入面
	var one int64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT 1 FROM "+table+" WHERE id=? AND family_id=?", id, familyID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "not_found", "资源不存在")
		return false
	}
	if err != nil {
		writeInternal(w, r, err, "校验资源失败")
		return false
	}
	return true
}

func (s *server) reloadFamily(w http.ResponseWriter, r *http.Request, familyID int64) {
	if err := s.deps.Store.ReloadFamily(s.deps.DBs, familyID); err != nil {
		slog.Error("重载家庭快照失败", "err", err, "family_id", familyID)
	}
}
