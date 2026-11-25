package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"good-habit/internal/ledger"
	"good-habit/internal/store"
)

const cookieName = "gh_session"

// sessionCtxKey 是会话在 context 里的键类型。
type sessionCtxKey struct{}

// SessionFrom 取当前请求的会话（由 chain 的会话加载中间件注入）。
func SessionFrom(r *http.Request) (store.Session, bool) {
	sess, ok := r.Context().Value(sessionCtxKey{}).(store.Session)
	return sess, ok
}

// RequireRole 构造角色门：未登录 401 unauthenticated；角色不符 403 forbidden。
func RequireRole(role string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := SessionFrom(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthenticated", "请先登录")
			return
		}
		if sess.Role != role {
			writeErr(w, http.StatusForbidden, "forbidden", "没有权限执行此操作")
			return
		}
		h(w, r)
	}
}

// setSessionCookie 写 30 天会话 Cookie（HttpOnly + SameSite=Strict + Path=/）；
// secure 由 Cfg.SecureCookie 传入（TLS 前置部署时开启）。
func setSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 3600,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
	})
}

// clearSessionCookie 过期会话 Cookie。
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// chain 是中间件链：请求ID → 访问日志 → 全局限流 → 会话加载 → next。
func (s *server) chain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := newRequestID()
		w.Header().Set("X-Request-Id", reqID)

		ip := s.clientIP(r)
		if !s.generalLimiter.Allow(ip) {
			writeErr(w, http.StatusTooManyRequests, "rate_limited", "请求太频繁，稍后再试")
			return
		}

		if c, err := r.Cookie(cookieName); err == nil && c.Value != "" {
			hash := sha256Hex(c.Value)
			if sess, ok := s.deps.Store.GetSession(hash); ok && sess.ExpiresAt > time.Now().UnixMilli() {
				r = r.WithContext(context.WithValue(r.Context(), sessionCtxKey{}, sess))
				// 滑动续期（spec §10：30 天滑动）：剩余不足 29 天时顺延到 30 天；
				// 续期后剩余回到 30 天，天然按天节流，不会每请求都写。
				nowMs := time.Now().UnixMilli()
				if sess.ExpiresAt < nowMs+29*24*3600*1000 {
					sess.ExpiresAt = nowMs + 30*24*3600*1000
					s.deps.Store.PutSession(sess)
					if s.deps.Enqueue != nil {
						s.deps.Enqueue(ledger.SessionUpsertOp{
							TokenHash: sess.TokenHash, UserID: sess.UserID,
							ExpiresAt: sess.ExpiresAt, CreatedAt: nowMs,
						})
					}
				}
			}
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http",
			"id", reqID, "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "dur", time.Since(start).String(), "ip", ip)
	})
}

// clientIP 提取客户端 IP。
// TrustProxy 时：优先 CF-Connecting-IP（Cloudflare Tunnel 的权威头）；
// 否则取 X-Forwarded-For 最右侧一跳（由离我们最近的可信代理追加，
// 首段可被客户端伪造——不能用作限流 key）。
func (s *server) clientIP(r *http.Request) string {
	if s.deps.Cfg.TrustProxy {
		if cip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cip != "" {
			return cip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// statusRecorder 记录响应码供访问日志使用。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
