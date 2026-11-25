package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// apiPath 拼接 /api/... 路径：apiPath("children", 12) → "/api/children/12"。
func apiPath(parts ...any) string {
	var sb strings.Builder
	sb.WriteString("/api")
	for _, p := range parts {
		sb.WriteString("/")
		sb.WriteString(fmt.Sprint(p))
	}
	return sb.String()
}

func parentCookie(sessToken string) *http.Cookie {
	return &http.Cookie{Name: "gh_session", Value: sessToken}
}

func TestChildLifecycle(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	parent, err := svc.Register("parent1", "password8", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	pc := parentCookie(parent.Token)

	// 创建
	rr := doJSON(h, "POST", "/api/children", map[string]any{
		"username": "kid001", "password": "password8", "displayName": "娃",
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("create child = %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Balance     int64  `json:"balance"`
		Active      bool   `json:"active"`
	}
	json.Unmarshal(rr.Body.Bytes(), &created)
	if created.ID == 0 || created.Username != "kid001" || created.DisplayName != "娃" ||
		created.Balance != 0 || !created.Active {
		t.Errorf("create 响应不符: %+v", created)
	}

	// 列表
	rr = doJSON(h, "GET", "/api/children", nil, pc)
	if rr.Code != 200 {
		t.Fatalf("list = %d", rr.Code)
	}
	var list struct {
		Children []struct {
			ID      int64 `json:"id"`
			Balance int64 `json:"balance"`
			Active  bool  `json:"active"`
		} `json:"children"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Children) != 1 || list.Children[0].ID != created.ID {
		t.Fatalf("列表不符: %+v", list.Children)
	}
	if deps.Store.Child(created.ID).Bal() != 0 {
		t.Error("store.Child 应就绪且余额为 0")
	}

	// 改名
	rr = doJSON(h, "PATCH", "/api/children/999999999", map[string]any{"displayName": "二娃"}, pc)
	_ = rr // 先确认不存在的 id
	if rr.Code != 404 {
		t.Fatalf("不存在 id 应 404, got %d", rr.Code)
	}
	rr = doJSON(h, "PATCH", apiPath("children", created.ID), map[string]any{"displayName": "二娃"}, pc)
	if rr.Code != 200 {
		t.Fatalf("rename = %d %s", rr.Code, rr.Body.String())
	}
	var renamed struct{ DisplayName string `json:"displayName"` }
	json.Unmarshal(rr.Body.Bytes(), &renamed)
	if renamed.DisplayName != "二娃" {
		t.Errorf("改名后 = %q, want 二娃", renamed.DisplayName)
	}

	// 重置密码：旧 401 新 200
	rr = doJSON(h, "PATCH", apiPath("children", created.ID), map[string]any{"password": "newpass99"}, pc)
	if rr.Code != 200 {
		t.Fatalf("reset pw = %d", rr.Code)
	}
	if rr := doJSON(h, "POST", "/api/auth/login", map[string]any{"username": "kid001", "password": "password8"}); rr.Code != 401 {
		t.Errorf("旧密码登录 = %d, want 401", rr.Code)
	}
	rr = doJSON(h, "POST", "/api/auth/login", map[string]any{"username": "kid001", "password": "newpass99"})
	if rr.Code != 200 {
		t.Fatalf("新密码登录 = %d %s", rr.Code, rr.Body.String())
	}
	childTok := sessionCookie(rr)
	if childTok == nil {
		t.Fatal("孩子登录应有 cookie")
	}
	if rr := doJSON(h, "GET", "/api/me", nil, childTok); rr.Code != 200 {
		t.Fatalf("孩子 me = %d", rr.Code)
	}

	// 停用：登录 409、列表 active=false、会话被清
	rr = doJSON(h, "PATCH", apiPath("children", created.ID), map[string]any{"active": false}, pc)
	if rr.Code != 200 {
		t.Fatalf("deactivate = %d %s", rr.Code, rr.Body.String())
	}
	if rr := doJSON(h, "POST", "/api/auth/login", map[string]any{"username": "kid001", "password": "newpass99"}); rr.Code != 409 || errCode(t, rr) != "account_inactive" {
		t.Errorf("停用后登录 = %d %s, want 409 account_inactive", rr.Code, rr.Body.String())
	}
	if _, ok := deps.Store.GetSession(sha256Hex(childTok.Value)); ok {
		t.Error("停用应清掉孩子的内存会话")
	}
	if rr := doJSON(h, "GET", "/api/me", nil, childTok); rr.Code != 401 {
		t.Errorf("停用后旧 cookie me = %d, want 401", rr.Code)
	}
	rr = doJSON(h, "GET", "/api/children", nil, pc)
	var list2 struct {
		Children []struct {
			Active bool `json:"active"`
		} `json:"children"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list2)
	if len(list2.Children) != 1 || list2.Children[0].Active {
		t.Errorf("停用后列表 active 应为 false: %+v", list2.Children)
	}

	// 校验失败 422
	if rr := doJSON(h, "POST", "/api/children", map[string]any{
		"username": "x", "password": "password8", "displayName": "娃",
	}, pc); rr.Code != 422 {
		t.Errorf("坏用户名 = %d, want 422", rr.Code)
	}
}

func TestChildFamilyIsolation(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	pa, _ := svc.Register("parenta", "password8", "")
	pb, _ := svc.Register("parentb", "password8", "")

	rr := doJSON(h, "POST", "/api/children", map[string]any{
		"username": "kid_a", "password": "password8", "displayName": "A娃",
	}, parentCookie(pa.Token))
	if rr.Code != 200 {
		t.Fatalf("create = %d", rr.Code)
	}
	var child struct{ ID int64 `json:"id"` }
	json.Unmarshal(rr.Body.Bytes(), &child)

	// B 家家长 PATCH A 家孩子 → 404
	rr = doJSON(h, "PATCH", apiPath("children", child.ID), map[string]any{"displayName": "偷改"}, parentCookie(pb.Token))
	if rr.Code != 404 {
		t.Errorf("跨家庭 PATCH = %d, want 404", rr.Code)
	}
	// B 家家长看不到 A 家孩子
	rr = doJSON(h, "GET", "/api/children", nil, parentCookie(pb.Token))
	var list struct {
		Children []json.RawMessage `json:"children"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Children) != 0 {
		t.Errorf("B 家应看不到孩子: %d", len(list.Children))
	}
	// 孩子自己的会话不能管理孩子
	rr = doJSON(h, "POST", "/api/auth/login", map[string]any{"username": "kid_a", "password": "password8"})
	kidCookie := sessionCookie(rr)
	rr = doJSON(h, "GET", "/api/children", nil, kidCookie)
	if rr.Code != 403 {
		t.Errorf("孩子访问家长路由 = %d, want 403", rr.Code)
	}
}
