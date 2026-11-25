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
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Points int64  `json:"points"`
	Active bool   `json:"active"`
	Sort   int64  `json:"sort"`
}

// GET /api/reward-items：管理视图（含停用项）。
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
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/reward-items：新增积分项。
func (s *server) handleItemCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		Title  string `json:"title"`
		Points int64  `json:"points"`
		Sort   int64  `json:"sort"`
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

	var id int64
	err := s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"INSERT INTO reward_items (family_id, title, points, sort, created_at) VALUES (?,?,?,?,?)",
			sess.FamilyID, body.Title, body.Points, body.Sort, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		writeInternal(w, r, err, "新建积分项失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writeItemView(w, r, id)
}

// PATCH /api/reward-items/{id}：改名/改分值/排序/启停。
func (s *server) handleItemUpdate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "ID 不合法")
		return
	}
	var body struct {
		Title  *string `json:"title"`
		Points int64   `json:"points"`
		Active *bool   `json:"active"`
		Sort   *int64  `json:"sort"`
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
	writeJSON(w, http.StatusOK, v)
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
