package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"good-habit/internal/config"
	"good-habit/internal/db"
)

// openProbe 打开独立只读探测连接（停机后核对落盘状态）。
func openProbe(dbPath string) (*db.DBs, error) {
	return db.Open(dbPath, 2)
}

func postJSON(h http.Handler, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func getJSON(h http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func cookieOf(rr *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == "gh_session" {
			return c
		}
	}
	return nil
}

func newCfg(t *testing.T) (config.Config, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "t.db")
	return config.Config{Addr: "127.0.0.1:0", DBPath: dbPath, ReadPool: 5}, dbPath
}

func TestAppLifecycleAndDrain(t *testing.T) {
	cfg, dbPath := newCfg(t)
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := a.Handler()

	rr := postJSON(h, "/api/auth/register", map[string]any{
		"username": "parent17", "password": "password8",
	})
	if rr.Code != 200 {
		t.Fatalf("register = %d %s", rr.Code, rr.Body.String())
	}
	pc := cookieOf(rr)

	rr = postJSON(h, "/api/children", map[string]any{
		"username": "kid17001", "password": "password8", "displayName": "娃",
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("create child = %d %s", rr.Code, rr.Body.String())
	}
	var child struct{ ID int64 `json:"id"` }
	json.Unmarshal(rr.Body.Bytes(), &child)

	rr = postJSON(h, "/api/reward-items", map[string]any{
		"title": "早读10分钟", "points": 5,
	}, pc)
	var item struct{ ID int64 `json:"id"` }
	json.Unmarshal(rr.Body.Bytes(), &item)

	rr = postJSON(h, "/api/earnings", map[string]any{
		"childId": child.ID, "rewardItemId": item.ID,
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("earn = %d %s", rr.Code, rr.Body.String())
	}

	// 优雅停机：Drain 后账单/会话必须已落盘
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	var nLedger, nSessions int64
	probe, err := openProbe(dbPath)
	if err != nil {
		t.Fatalf("openProbe: %v", err)
	}
	defer probe.Close()
	probe.Read.QueryRow("SELECT COUNT(*) FROM ledger WHERE child_id=?", child.ID).Scan(&nLedger)
	probe.Read.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&nSessions)
	if nLedger != 1 {
		t.Errorf("Drain 后 ledger 行 = %d, want 1", nLedger)
	}
	if nSessions != 1 {
		t.Errorf("Drain 后 sessions 行 = %d, want 1", nSessions)
	}
}

func TestAppRestart(t *testing.T) {
	cfg, _ := newCfg(t)
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := a.Handler()

	rr := postJSON(h, "/api/auth/register", map[string]any{
		"username": "parent18", "password": "password8",
	})
	pc := cookieOf(rr)
	rr = postJSON(h, "/api/children", map[string]any{
		"username": "kid18001", "password": "password8", "displayName": "娃",
	}, pc)
	var child struct{ ID int64 `json:"id"` }
	json.Unmarshal(rr.Body.Bytes(), &child)
	rr = postJSON(h, "/api/reward-items", map[string]any{"title": "项", "points": 7}, pc)
	var item struct{ ID int64 `json:"id"` }
	json.Unmarshal(rr.Body.Bytes(), &item)
	postJSON(h, "/api/earnings", map[string]any{
		"childId": child.ID, "rewardItemId": item.ID,
	}, pc)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// 重启：会话与余额都恢复
	a2, err := New(cfg)
	if err != nil {
		t.Fatalf("二次 New: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = a2.Shutdown(ctx)
	}()
	h2 := a2.Handler()
	if rr := getJSON(h2, "/api/me", pc); rr.Code != 200 {
		t.Errorf("重启后旧 cookie me = %d, want 200（会话应恢复）", rr.Code)
	}
	if bal := a2.store.Child(child.ID).Bal(); bal != 7 {
		t.Errorf("重启后余额 = %d, want 7", bal)
	}
}

func TestResetPassword(t *testing.T) {
	cfg, dbPath := newCfg(t)
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := a.Handler()
	postJSON(h, "/api/auth/register", map[string]any{
		"username": "parent19", "password": "password8",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	a.Shutdown(ctx)

	probe, err := openProbe(dbPath)
	if err != nil {
		t.Fatalf("openProbe: %v", err)
	}
	defer probe.Close()
	if err := ResetPassword(probe, "parent19", "brandnew9"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	// 短密码 → ErrValidation
	if err := ResetPassword(probe, "parent19", "short"); err == nil {
		t.Error("短密码应报错")
	}

	// 新密码可登录（起一个新 app 验证）
	a2, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = a2.Shutdown(ctx)
	}()
	rr := postJSON(a2.Handler(), "/api/auth/login", map[string]any{
		"username": "parent19", "password": "brandnew9",
	})
	if rr.Code != 200 {
		t.Fatalf("新密码登录 = %d %s", rr.Code, rr.Body.String())
	}
	rr = postJSON(a2.Handler(), "/api/auth/login", map[string]any{
		"username": "parent19", "password": "password8",
	})
	if rr.Code != 401 {
		t.Errorf("旧密码登录 = %d, want 401", rr.Code)
	}
}
