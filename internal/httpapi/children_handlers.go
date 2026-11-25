package httpapi

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"good-habit/internal/auth"
)

// childView 是孩子管理 API 的响应形状。
type childView struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Balance     int64  `json:"balance"`
	Active      bool   `json:"active"`
}

// GET /api/children：本家庭全部孩子（含停用，余额读内存）。
func (s *server) handleChildrenList(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	rows, err := s.deps.DBs.Read.Query(
		"SELECT id, username, display_name, active FROM users WHERE family_id=? AND role='child' ORDER BY id",
		sess.FamilyID)
	if err != nil {
		writeInternal(w, r, err, "查孩子失败")
		return
	}
	defer rows.Close()
	views := []childView{}
	for rows.Next() {
		var v childView
		var active int64
		if err := rows.Scan(&v.ID, &v.Username, &v.DisplayName, &active); err != nil {
			writeInternal(w, r, err, "读孩子失败")
			return
		}
		v.Active = active == 1
		v.Balance = s.deps.Store.Child(v.ID).Bal()
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"children": views})
}

// POST /api/children：建孩子子账号。
func (s *server) handleChildCreate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !auth.ValidUsername(body.Username) {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_username", "用户名需 3-32 位小写字母、数字或下划线")
		return
	}
	if len(body.Password) < 8 {
		writeErr(w, http.StatusUnprocessableEntity, "short_password", "密码至少 8 位")
		return
	}
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.DisplayName == "" {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_display_name", "请填写孩子昵称")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeInternal(w, r, err, "密码散列失败")
		return
	}
	now := time.Now().UnixMilli()
	var childID int64
	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO users (family_id, role, username, password_hash, display_name, created_at)
			 VALUES (?,?,?,?,?,?)`,
			sess.FamilyID, "child", strings.ToLower(body.Username), hash, body.DisplayName, now)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return auth.ErrDuplicateUsername
			}
			return err
		}
		childID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		s.writeAuthErr(w, err)
		return
	}
	s.deps.Store.Child(childID) // 就绪零值账户
	writeJSON(w, http.StatusOK, childView{
		ID: childID, Username: strings.ToLower(body.Username),
		DisplayName: body.DisplayName, Balance: 0, Active: true,
	})
}

// PATCH /api/children/{id}：改名/重置密码/停用（软删除）。
func (s *server) handleChildUpdate(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r)
	childID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "孩子 ID 不合法")
		return
	}
	var body struct {
		DisplayName *string `json:"displayName"`
		Password    string  `json:"password"`
		Active      *bool   `json:"active"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DisplayName != nil && strings.TrimSpace(*body.DisplayName) == "" {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_display_name", "昵称不能为空")
		return
	}
	if body.Password != "" && len(body.Password) < 8 {
		writeErr(w, http.StatusUnprocessableEntity, "short_password", "密码至少 8 位")
		return
	}

	// 归属校验（跨家庭一律 404）
	var uname, disp string
	var active int64
	err = s.deps.DBs.Read.QueryRow(
		"SELECT username, display_name, active FROM users WHERE id=? AND family_id=? AND role='child'",
		childID, sess.FamilyID,
	).Scan(&uname, &disp, &active)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "not_found", "孩子不存在")
		return
	}
	if err != nil {
		writeInternal(w, r, err, "查孩子失败")
		return
	}

	var newHash *string
	if body.Password != "" {
		h, err := auth.HashPassword(body.Password)
		if err != nil {
			writeInternal(w, r, err, "密码散列失败")
			return
		}
		newHash = &h
	}
	deactivate := body.Active != nil && !*body.Active

	err = s.deps.DBs.WithTx(func(tx *sql.Tx) error {
		if body.DisplayName != nil {
			if _, err := tx.Exec("UPDATE users SET display_name=? WHERE id=?", strings.TrimSpace(*body.DisplayName), childID); err != nil {
				return err
			}
		}
		if newHash != nil {
			if _, err := tx.Exec("UPDATE users SET password_hash=? WHERE id=?", *newHash, childID); err != nil {
				return err
			}
			// 重置密码即踢下线（与停用同一安全语义）
			if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=?", childID); err != nil {
				return err
			}
		}
		if body.Active != nil {
			v := int64(1)
			if !*body.Active {
				v = 0
			}
			if _, err := tx.Exec("UPDATE users SET active=? WHERE id=?", v, childID); err != nil {
				return err
			}
			if deactivate {
				// 停用即踢下线：清 DB 会话（内存态随后清）
				if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=?", childID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		writeInternal(w, r, err, "更新孩子失败")
		return
	}
	if deactivate {
		s.deps.Store.DeleteSessionsForUser(childID)
	}
	if newHash != nil {
		s.deps.Store.DeleteSessionsForUser(childID)
	}
	if body.Active != nil && *body.Active {
		// 恢复启用：停用期间无任何写入（孩子无会话、earn 校验 active=1），
		// DB 即权威——从 DB 水化热状态，防止重启后懒创建的零值账户覆盖真实余额。
		s.hydrateChild(childID)
	}

	// 回读构造响应（写后读，同步事务已提交）
	var v childView
	var act int64
	err = s.deps.DBs.Read.QueryRow(
		"SELECT id, username, display_name, active FROM users WHERE id=?", childID,
	).Scan(&v.ID, &v.Username, &v.DisplayName, &act)
	if err != nil {
		writeInternal(w, r, err, "回读孩子失败")
		return
	}
	v.Active = act == 1
	v.Balance = s.deps.Store.Child(childID).Bal()
	writeJSON(w, http.StatusOK, v)
}

// hydrateChild 从 DB 重建孩子的内存热状态（余额/待选局/待发放数）。
// 使用场景：恢复启用（停用期间无任何写入，DB 为唯一权威）。
func (s *server) hydrateChild(childID int64) {
	var bal int64
	if err := s.deps.DBs.Read.QueryRow(
		"SELECT balance FROM users WHERE id=?", childID).Scan(&bal); err != nil {
		slog.Error("水化孩子余额失败", "err", err, "child_id", childID)
	}
	var unful int64
	if err := s.deps.DBs.Read.QueryRow(
		"SELECT COUNT(*) FROM draws WHERE child_id=? AND state='completed' AND fulfilled_at IS NULL",
		childID).Scan(&unful); err != nil {
		slog.Error("水化待发放数失败", "err", err, "child_id", childID)
	}
	var pendUUID string
	var pendDeadline sql.NullInt64
	_ = s.deps.DBs.Read.QueryRow(
		"SELECT uuid, created_at + 172800000 FROM draws WHERE child_id=? AND state='pending_choice' LIMIT 1",
		childID).Scan(&pendUUID, &pendDeadline)

	c := s.deps.Store.Child(childID)
	c.Lock()
	defer c.Unlock()
	c.Balance = bal
	c.Unfulfilled = unful
	c.PendingUUID = pendUUID
	c.PendingDeadline = 0
	if pendDeadline.Valid {
		c.PendingDeadline = pendDeadline.Int64
	}
}
