package httpapi

import (
	"errors"
	"net/http"

	"good-habit/internal/auth"
	"good-habit/internal/store"
)

// POST /api/auth/register：注册家长账号并建立会话。
func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.authLimiter.Allow(s.clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "rate_limited", "尝试太频繁，稍后再试")
		return
	}
	var body struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		FamilyName string `json:"family_name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if !auth.ValidUsername(body.Username) {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_username",
			"用户名需 3-32 位小写字母、数字或下划线")
		return
	}
	if len(body.Password) < 8 {
		writeErr(w, http.StatusUnprocessableEntity, "short_password", "密码至少 8 位")
		return
	}

	sess, err := s.deps.Auth.Register(body.Username, body.Password, body.FamilyName)
	if err != nil {
		s.writeAuthErr(w, err)
		return
	}
	s.finishLogin(w, sess)
}

// POST /api/auth/login：登录（家长与孩子同一入口）。
func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.authLimiter.Allow(s.clientIP(r)) {
		writeErr(w, http.StatusTooManyRequests, "rate_limited", "尝试太频繁，稍后再试")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	sess, err := s.deps.Auth.Login(body.Username, body.Password)
	if err != nil {
		s.writeAuthErr(w, err)
		return
	}
	s.finishLogin(w, sess)
}

// POST /api/auth/logout：登出（幂等）。
func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := SessionFrom(r); ok {
		s.deps.Auth.Logout(sess.TokenHash)
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/me：当前用户信息。
func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := SessionFrom(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated", "请先登录")
		return
	}
	user, ok := s.userPayload(sess.UserID)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthenticated", "会话已失效")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *server) finishLogin(w http.ResponseWriter, sess store.Session) {
	setSessionCookie(w, sess.Token, s.deps.Cfg.SecureCookie)
	user, ok := s.userPayload(sess.UserID)
	if !ok {
		// 会话刚建立，用户行必然存在；防御性兜底
		writeErr(w, http.StatusInternalServerError, "internal", "建会话后查不到用户")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *server) userPayload(userID int64) (map[string]any, bool) {
	var username, display, role string
	var familyID int64
	err := s.deps.DBs.Read.QueryRow(
		"SELECT username, display_name, role, family_id FROM users WHERE id=? AND active=1",
		userID,
	).Scan(&username, &display, &role, &familyID)
	if err != nil {
		return nil, false
	}
	return map[string]any{
		"id": userID, "username": username, "displayName": display,
		"role": role, "familyId": familyID,
	}, true
}

func (s *server) writeAuthErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrDuplicateUsername):
		writeErr(w, http.StatusConflict, "duplicate_username", "用户名已被占用")
	case errors.Is(err, auth.ErrValidation):
		writeErr(w, http.StatusUnprocessableEntity, "invalid_input", "输入不合法")
	case errors.Is(err, auth.ErrBadCredentials):
		writeErr(w, http.StatusUnauthorized, "bad_credentials", "用户名或密码不对")
	case errors.Is(err, auth.ErrLocked):
		writeErr(w, http.StatusConflict, "account_locked", "失败次数太多，请稍后再试")
	case errors.Is(err, auth.ErrInactive):
		writeErr(w, http.StatusConflict, "account_inactive", "账号已停用")
	default:
		writeErr(w, http.StatusInternalServerError, "internal", "服务器开小差了")
	}
}
