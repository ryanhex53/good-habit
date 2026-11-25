package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"good-habit/internal/auth"
	"good-habit/internal/config"
	"good-habit/internal/db"
	"good-habit/internal/ledger"
	"good-habit/internal/store"
	"good-habit/migrations"
)

func newDeps(t *testing.T) (Deps, *auth.Service) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st := store.New()
	svc := &auth.Service{DBs: dbs, Store: st, Enqueue: func(ledger.Op) {}, LockWindow: time.Minute}
	return Deps{
		Cfg:   config.Config{},
		DBs:   dbs,
		Store: st,
		Auth:  svc,
		Enqueue: func(ledger.Op) {},
	}, svc
}

func putSession(t *testing.T, st *store.Store, token, role string) string {
	t.Helper()
	hash := sha256Hex(token)
	st.PutSession(store.Session{
		TokenHash: hash, UserID: 1, FamilyID: 1, Role: role,
		ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
	})
	return token
}

func ok200(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestHealthz(t *testing.T) {
	deps, _ := newDeps(t)
	h := New(deps)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if rr.Code != 200 {
		t.Errorf("/healthz = %d, want 200", rr.Code)
	}
}

func TestUnauthEnvelope(t *testing.T) {
	deps, _ := newDeps(t)
	s := newServer(deps)
	mux := s.routes()
	mux.HandleFunc("GET /api/dummy", RequireRole("parent", ok200))
	h := s.chain(mux)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/dummy", nil))
	if rr.Code != 401 {
		t.Errorf("code = %d, want 401", rr.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是 JSON 信封: %v", err)
	}
	if body.Error.Code != "unauthenticated" {
		t.Errorf("code = %q, want unauthenticated", body.Error.Code)
	}
	if rr.Header().Get("X-Request-Id") == "" {
		t.Error("应带 X-Request-Id 头")
	}
}

func TestRoleGate(t *testing.T) {
	deps, svc := newDeps(t)
	parent, err := svc.Register("parent1", "password8", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	childTok := putSession(t, deps.Store, "child-tok", "child")

	s := newServer(deps)
	mux := s.routes()
	mux.HandleFunc("GET /api/child-only", RequireRole("child", ok200))
	mux.HandleFunc("GET /api/parent-only", RequireRole("parent", ok200))
	h := s.chain(mux)

	do := func(path, cookie string) int {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: "gh_session", Value: cookie})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}
	if got := do("/api/child-only", parent.Token); got != 403 {
		t.Errorf("parent 访问 child 路由 = %d, want 403", got)
	}
	if got := do("/api/parent-only", childTok); got != 403 {
		t.Errorf("child 访问 parent 路由 = %d, want 403", got)
	}
	if got := do("/api/parent-only", parent.Token); got != 200 {
		t.Errorf("parent 访问自己的路由 = %d, want 200", got)
	}
	if got := do("/api/child-only", childTok); got != 200 {
		t.Errorf("child 访问自己的路由 = %d, want 200", got)
	}
}

func TestRateLimitGeneral(t *testing.T) {
	deps, _ := newDeps(t)
	deps.RateGeneral = 3
	h := New(deps)

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
		if rr.Code != 200 {
			t.Fatalf("第 %d 次 = %d, want 200", i+1, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if rr.Code != 429 {
		t.Errorf("第 4 次 = %d, want 429", rr.Code)
	}
	var body struct {
		Error struct{ Code string `json:"code"` } `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Error.Code != "rate_limited" {
		t.Errorf("第 4 次响应 = %q err=%v, want rate_limited", rr.Body.String(), err)
	}
}

func TestClientIPUsesRightmostAndCF(t *testing.T) {
	// 用限流计数作为 clientIP 的可观测面
	fresh := func() http.Handler {
		deps, _ := newDeps(t)
		deps.Cfg.TrustProxy = true
		deps.RateGeneral = 2
		return New(deps)
	}
	get := func(h http.Handler, xff, cip string) int {
		req := httptest.NewRequest("GET", "/healthz", nil)
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		if cip != "" {
			req.Header.Set("CF-Connecting-IP", cip)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// 同一真实来源（XFF 右段）+ 每次不同伪造首段：应仍被同 key 限流
	h := fresh()
	if got := get(h, "spoof1, 1.2.3.4", ""); got != 200 {
		t.Fatalf("第 1 次 = %d", got)
	}
	if got := get(h, "spoof2, 1.2.3.4", ""); got != 200 {
		t.Fatalf("第 2 次 = %d", got)
	}
	if got := get(h, "spoof3, 1.2.3.4", ""); got != 429 {
		t.Errorf("伪造首段不应绕过限流（按右段计数）= %d, want 429", got)
	}

	// CF-Connecting-IP 优先于 XFF
	h2 := fresh()
	get(h2, "spoof1, 1.2.3.4", "6.6.6.6")
	get(h2, "spoof2, 1.2.3.4", "6.6.6.6")
	if got := get(h2, "spoof3, 1.2.3.4", "6.6.6.6"); got != 429 {
		t.Errorf("CF-Connecting-IP 应作为限流 key = %d, want 429", got)
	}
	// 不同 CF IP 同 XFF → 不受限流牵连
	if got := get(h2, "spoof4, 1.2.3.4", "7.7.7.7"); got != 200 {
		t.Errorf("不同 CF IP = %d, want 200", got)
	}
}

func TestCookieAttrs(t *testing.T) {
	rr := httptest.NewRecorder()
	setSessionCookie(rr, "tok123", false)
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie 数 = %d, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != "gh_session" || c.Value != "tok123" {
		t.Errorf("cookie = %s=%s, want gh_session=tok123", c.Name, c.Value)
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 2592000 {
		t.Errorf("cookie 属性不符: HttpOnly=%v SameSite=%v Path=%q MaxAge=%d",
			c.HttpOnly, c.SameSite, c.Path, c.MaxAge)
	}

	rr2 := httptest.NewRecorder()
	clearSessionCookie(rr2)
	cs := rr2.Result().Cookies()
	if len(cs) != 1 || cs[0].Name != "gh_session" || cs[0].MaxAge >= 0 {
		t.Errorf("清 cookie 应 MaxAge<0: %+v", cs)
	}
}

func TestSessionSlidingRenewal(t *testing.T) {
	dbs := newDBForMiddleware(t)
	st := store.New()
	ops := make(chan ledger.Op, 8)
	svc := &auth.Service{DBs: dbs, Store: st, Enqueue: func(op ledger.Op) { ops <- op }, LockWindow: time.Minute}
	h := New(Deps{Cfg: config.Config{}, DBs: dbs, Store: st, Auth: svc, Enqueue: func(op ledger.Op) { ops <- op }})

	hash := sha256Hex("sliding-tok")
	dayMs := int64(24 * 3600 * 1000)
	now := time.Now().UnixMilli()
	// 老会话：只剩 2 天
	st.PutSession(store.Session{TokenHash: hash, UserID: 1, FamilyID: 1, Role: "child", ExpiresAt: now + 2*dayMs})

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.AddCookie(&http.Cookie{Name: "gh_session", Value: "sliding-tok"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("healthz = %d", rr.Code)
	}

	got, ok := st.GetSession(hash)
	if !ok {
		t.Fatal("会话应仍在")
	}
	if got.ExpiresAt < now+29*dayMs {
		t.Errorf("老会话未滑动续期：ExpiresAt 仅剩 %d 天", (got.ExpiresAt-now)/dayMs)
	}
	select {
	case op := <-ops:
		up, ok := op.(ledger.SessionUpsertOp)
		if !ok || up.TokenHash != hash || up.ExpiresAt != got.ExpiresAt {
			t.Errorf("续期应入队 SessionUpsertOp: %+v", op)
		}
	default:
		t.Error("续期应入队落盘事件")
	}

	// 剩余 > 29 天的新会话不重复续（天然按天节流）
	hash2 := sha256Hex("fresh-tok")
	st.PutSession(store.Session{TokenHash: hash2, UserID: 1, FamilyID: 1, Role: "child", ExpiresAt: now + 30*dayMs - 3600*1000})
	req2 := httptest.NewRequest("GET", "/healthz", nil)
	req2.AddCookie(&http.Cookie{Name: "gh_session", Value: "fresh-tok"})
	h.ServeHTTP(httptest.NewRecorder(), req2)
	got2, _ := st.GetSession(hash2)
	if got2.ExpiresAt != now+30*dayMs-3600*1000 {
		t.Errorf("新会话不应被续期扰动")
	}
}

// newDBForMiddleware 提供中间件测试用的 DB（迁移后）。
func newDBForMiddleware(t *testing.T) *db.DBs {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return dbs
}
