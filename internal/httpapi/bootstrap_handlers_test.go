package httpapi

import (
	"encoding/json"
	"testing"
)

func TestBootstrapDefault(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "booter1")
	pc := parentCookie(p.Token)

	rr := doJSON(h, "POST", "/api/bootstrap", map[string]any{}, pc)
	if rr.Code != 200 {
		t.Fatalf("bootstrap = %d %s", rr.Code, rr.Body.String())
	}
	var res struct {
		ImportedItems  int64 `json:"importedItems"`
		ImportedPrizes int64 `json:"importedPrizes"`
	}
	json.Unmarshal(rr.Body.Bytes(), &res)
	if res.ImportedItems != 11 || res.ImportedPrizes != 3 {
		t.Errorf("导入数 = %d/%d, want 11/3", res.ImportedItems, res.ImportedPrizes)
	}

	var ni, np int64
	deps.DBs.Read.QueryRow(
		"SELECT COUNT(*) FROM reward_items WHERE family_id=?", p.FamilyID).Scan(&ni)
	deps.DBs.Read.QueryRow(
		"SELECT COUNT(*) FROM prizes WHERE family_id=?", p.FamilyID).Scan(&np)
	if ni != 11 || np != 3 {
		t.Errorf("DB 行数 = %d/%d, want 11/3", ni, np)
	}

	// 快照同步（11 项 3 奖、默认定价 5/7）
	fc := deps.Store.Family(p.FamilyID)
	if fc == nil || len(fc.Items) != 11 || len(fc.Prizes) != 3 || fc.SingleCost != 5 || fc.DoubleCost != 7 {
		t.Errorf("快照不符: items=%d prizes=%d costs=%d/%d",
			len(fc.Items), len(fc.Prizes), fc.SingleCost, fc.DoubleCost)
	}

	// 二次导入 → 409
	rr = doJSON(h, "POST", "/api/bootstrap", map[string]any{}, pc)
	if rr.Code != 409 || errCode(t, rr) != "already_bootstrapped" {
		t.Errorf("二次导入 = %d %s, want 409 already_bootstrapped", rr.Code, rr.Body.String())
	}
}

func TestBootstrapCustom(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "booter2")
	pc := parentCookie(p.Token)

	rr := doJSON(h, "POST", "/api/bootstrap", map[string]any{
		"items":      []map[string]any{{"title": "自定义项", "points": 3}},
		"prizes":     []map[string]any{{"name": "贴纸一张", "emoji": "✨"}},
		"singleCost": 6,
		"doubleCost": 9,
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("custom bootstrap = %d %s", rr.Code, rr.Body.String())
	}
	fc := deps.Store.Family(p.FamilyID)
	if fc == nil || len(fc.Items) != 1 || len(fc.Prizes) != 1 || fc.SingleCost != 6 || fc.DoubleCost != 9 {
		t.Errorf("自定义导入快照不符: %+v", fc)
	}
	if fc.Items[0].Title != "自定义项" || fc.Prizes[0].Emoji != "✨" {
		t.Errorf("自定义内容不符: %+v %+v", fc.Items[0], fc.Prizes[0])
	}
}

func TestBootstrapChildForbidden(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "booter3")
	pc := parentCookie(p.Token)
	if rr := doJSON(h, "POST", "/api/children", map[string]any{
		"username": "bootkid", "password": "password8", "displayName": "娃",
	}, pc); rr.Code != 200 {
		t.Fatalf("建孩子 = %d", rr.Code)
	}
	rr := doJSON(h, "POST", "/api/auth/login", map[string]any{
		"username": "bootkid", "password": "password8",
	})
	kid := sessionCookie(rr)
	rr = doJSON(h, "POST", "/api/bootstrap", map[string]any{}, kid)
	if rr.Code != 403 {
		t.Errorf("孩子 bootstrap = %d, want 403", rr.Code)
	}
}
