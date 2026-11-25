package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func doJSON(h http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func sessionCookie(rr *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == "gh_session" {
			return c
		}
	}
	return nil
}

func errCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("非 JSON 信封: %q", rr.Body.String())
	}
	return body.Error.Code
}

func TestAuthFlow(t *testing.T) {
	deps, _ := newDeps(t)
	h := New(deps)

	rr := doJSON(h, "POST", "/api/auth/register", map[string]any{
		"username": "dad01", "password": "password8", "family_name": "我家",
	})
	if rr.Code != 200 {
		t.Fatalf("register = %d %s", rr.Code, rr.Body.String())
	}
	var reg struct {
		User struct {
			ID          int64  `json:"id"`
			Username    string `json:"username"`
			DisplayName string `json:"displayName"`
			Role        string `json:"role"`
			FamilyID    int64  `json:"familyId"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reg); err != nil {
		t.Fatalf("register 响应: %v", err)
	}
	if reg.User.Username != "dad01" || reg.User.Role != "parent" || reg.User.FamilyID == 0 {
		t.Errorf("register user 不符: %+v", reg.User)
	}
	cookie := sessionCookie(rr)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("register 应下发 gh_session cookie")
	}

	rr = doJSON(h, "GET", "/api/me", nil, cookie)
	if rr.Code != 200 {
		t.Fatalf("me = %d %s", rr.Code, rr.Body.String())
	}
	var me struct {
		User struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"user"`
	}
	json.Unmarshal(rr.Body.Bytes(), &me)
	if me.User.ID != reg.User.ID || me.User.Username != "dad01" || me.User.Role != "parent" {
		t.Errorf("me 不符: %+v", me.User)
	}

	rr = doJSON(h, "POST", "/api/auth/login", map[string]any{
		"username": "DAD01", "password": "password8",
	})
	if rr.Code != 200 {
		t.Fatalf("login = %d %s", rr.Code, rr.Body.String())
	}

	rr = doJSON(h, "POST", "/api/auth/logout", nil, cookie)
	if rr.Code != 204 {
		t.Fatalf("logout = %d, want 204", rr.Code)
	}
	if c := sessionCookie(rr); c == nil || c.MaxAge >= 0 {
		t.Error("logout 应过期 cookie")
	}

	rr = doJSON(h, "GET", "/api/me", nil, cookie)
	if rr.Code != 401 {
		t.Errorf("登出后 me = %d, want 401", rr.Code)
	}
}

func TestRegisterErrors(t *testing.T) {
	deps, _ := newDeps(t)
	h := New(deps)

	rr := doJSON(h, "POST", "/api/auth/register", map[string]any{
		"username": "dad01", "password": "password8",
	})
	if rr.Code != 200 {
		t.Fatalf("首次注册 = %d", rr.Code)
	}
	rr = doJSON(h, "POST", "/api/auth/register", map[string]any{
		"username": "dad01", "password": "password8",
	})
	if rr.Code != 409 || errCode(t, rr) != "duplicate_username" {
		t.Errorf("重复注册 = %d %s, want 409 duplicate_username", rr.Code, rr.Body.String())
	}
	rr = doJSON(h, "POST", "/api/auth/register", map[string]any{
		"username": "okname", "password": "short",
	})
	if rr.Code != 422 || errCode(t, rr) != "short_password" {
		t.Errorf("短密码 = %d %s, want 422 short_password", rr.Code, rr.Body.String())
	}
	rr = doJSON(h, "POST", "/api/auth/register", map[string]any{
		"username": "坏 名字", "password": "password8",
	})
	if rr.Code != 422 || errCode(t, rr) != "invalid_username" {
		t.Errorf("坏用户名 = %d %s, want 422 invalid_username", rr.Code, rr.Body.String())
	}
}

func TestLoginErrors(t *testing.T) {
	deps, _ := newDeps(t)
	h := New(deps)
	if rr := doJSON(h, "POST", "/api/auth/register", map[string]any{
		"username": "kid001", "password": "password8",
	}); rr.Code != 200 {
		t.Fatalf("register = %d", rr.Code)
	}

	rr := doJSON(h, "POST", "/api/auth/login", map[string]any{
		"username": "kid001", "password": "wrong-pw-9",
	})
	if rr.Code != 401 || errCode(t, rr) != "bad_credentials" {
		t.Errorf("错密码 = %d %s, want 401 bad_credentials", rr.Code, rr.Body.String())
	}

	for i := 0; i < 4; i++ {
		doJSON(h, "POST", "/api/auth/login", map[string]any{
			"username": "kid001", "password": "wrong-pw-9",
		})
	}
	rr = doJSON(h, "POST", "/api/auth/login", map[string]any{
		"username": "kid001", "password": "password8",
	})
	if rr.Code != 409 || errCode(t, rr) != "account_locked" {
		t.Errorf("锁定后正确密码 = %d %s, want 409 account_locked", rr.Code, rr.Body.String())
	}
}
