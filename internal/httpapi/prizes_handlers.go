package httpapi

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type prizeView struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Emoji  string `json:"emoji"`
	Active bool   `json:"active"`
	Sort   int64  `json:"sort"`
}

// GET /api/prizes：管理视图（含停用）。
func (s *server) handlePrizesList(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	rows, err := s.deps.DBs.Read.Query(
		"SELECT id, name, emoji, active, sort FROM prizes WHERE family_id=? ORDER BY sort, id",
		sess.FamilyID)
	if err != nil {
		writeInternal(w, r, err, "查奖品失败")
		return
	}
	defer rows.Close()
	prizes := []prizeView{}
	for rows.Next() {
		var v prizeView
		var active int64
		if err := rows.Scan(&v.ID, &v.Name, &v.Emoji, &active, &v.Sort); err != nil {
			writeInternal(w, r, err, "读奖品失败")
			return
		}
		v.Active = active == 1
		prizes = append(prizes, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"prizes": prizes})
}

// POST /api/prizes：新增奖品（emoji 缺省 🎁）。
func (s *server) handlePrizeCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		Name  string `json:"name"`
		Emoji string `json:"emoji"`
		Sort  int64  `json:"sort"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_name", "请填写奖品名称")
		return
	}
	body.Emoji = strings.TrimSpace(body.Emoji)
	if body.Emoji == "" {
		body.Emoji = "🎁"
	}
	if len([]rune(body.Emoji)) > 8 {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_emoji", "emoji 最多 8 个字符")
		return
	}

	var id int64
	err := s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"INSERT INTO prizes (family_id, name, emoji, sort, created_at) VALUES (?,?,?,?,?)",
			sess.FamilyID, body.Name, body.Emoji, body.Sort, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		writeInternal(w, r, err, "新建奖品失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writePrizeView(w, r, id)
}

// PATCH /api/prizes/{id}：改名/emoji/排序/启停。
func (s *server) handlePrizeUpdate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "ID 不合法")
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Emoji  *string `json:"emoji"`
		Active *bool   `json:"active"`
		Sort   *int64  `json:"sort"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name != nil && strings.TrimSpace(*body.Name) == "" {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_name", "名称不能为空")
		return
	}
	if !s.owned(w, r, "prizes", id, sess.FamilyID) {
		return
	}
	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		if body.Name != nil {
			if _, err := tx.Exec("UPDATE prizes SET name=? WHERE id=?", strings.TrimSpace(*body.Name), id); err != nil {
				return err
			}
		}
		if body.Emoji != nil {
			e := strings.TrimSpace(*body.Emoji)
			if e == "" {
				e = "🎁"
			}
			if _, err := tx.Exec("UPDATE prizes SET emoji=? WHERE id=?", e, id); err != nil {
				return err
			}
		}
		if body.Sort != nil {
			if _, err := tx.Exec("UPDATE prizes SET sort=? WHERE id=?", *body.Sort, id); err != nil {
				return err
			}
		}
		if body.Active != nil {
			v := int64(0)
			if *body.Active {
				v = 1
			}
			if _, err := tx.Exec("UPDATE prizes SET active=? WHERE id=?", v, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		writeInternal(w, r, err, "更新奖品失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writePrizeView(w, r, id)
}

// DELETE /api/prizes/{id}：软删除。
func (s *server) handlePrizeDelete(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "ID 不合法")
		return
	}
	if !s.owned(w, r, "prizes", id, sess.FamilyID) {
		return
	}
	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE prizes SET active=0 WHERE id=?", id)
		return err
	})
	if err != nil {
		writeInternal(w, r, err, "删除奖品失败")
		return
	}
	s.reloadFamily(w, r, sess.FamilyID)
	s.writePrizeView(w, r, id)
}

func (s *server) writePrizeView(w http.ResponseWriter, r *http.Request, id int64) {
	var v prizeView
	var active int64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT id, name, emoji, active, sort FROM prizes WHERE id=?", id,
	).Scan(&v.ID, &v.Name, &v.Emoji, &active, &v.Sort)
	if err != nil {
		writeInternal(w, r, err, "回读奖品失败")
		return
	}
	v.Active = active == 1
	writeJSON(w, http.StatusOK, v)
}
