package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"good-habit/internal/auth"
	"good-habit/internal/store"
)

func regParent(t *testing.T, svc *auth.Service, username string) store.Session {
	t.Helper()
	sess, err := svc.Register(username, "password8", "")
	if err != nil {
		t.Fatalf("Register %s: %v", username, err)
	}
	return sess
}

type itemResp struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Points   int64   `json:"points"`
	Active   bool    `json:"active"`
	Sort     int64   `json:"sort"`
	ChildIDs []int64 `json:"childIds"`
}

func TestRewardItemCRUD(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "parent01")
	pc := parentCookie(p.Token)

	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
		"title": "托管课写完作业", "points": 7,
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("create = %d %s", rr.Code, rr.Body.String())
	}
	var item itemResp
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ID == 0 || item.Title != "托管课写完作业" || item.Points != 7 || !item.Active || item.ChildIDs == nil {
		t.Errorf("create 响应不符: %+v, childIds 应为 []", item)
	}

	// 改分值 + 排序
	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{
		"points": 10, "sort": 3,
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("patch = %d %s", rr.Code, rr.Body.String())
	}
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Points != 10 || item.Sort != 3 {
		t.Errorf("patch 后不符: %+v", item)
	}

	// 校验失败：非正分值
	rr = doJSON(h, "POST", "/api/reward-items", map[string]any{"title": "坏", "points": 0}, pc)
	if rr.Code != 422 {
		t.Errorf("points=0 = %d, want 422", rr.Code)
	}

	// 软删除后 GET 仍列出（active=false）
	rr = doJSON(h, "DELETE", apiPath("reward-items", item.ID), nil, pc)
	if rr.Code != 200 {
		t.Fatalf("delete = %d %s", rr.Code, rr.Body.String())
	}
	rr = doJSON(h, "GET", "/api/reward-items", nil, pc)
	var list struct {
		Items []itemResp `json:"items"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].Active {
		t.Fatalf("软删除后列表不符: %+v", list.Items)
	}
	// 再恢复
	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"active": true}, pc)
	if rr.Code != 200 {
		t.Fatalf("恢复 = %d", rr.Code)
	}
}

// mkScopedItem 建一个指定孩子的积分项（不传 childIDs 即所有孩子）。
func mkScopedItem(t *testing.T, h http.Handler, pc *http.Cookie, title string, points int64, childIDs ...int64) itemInfo {
	t.Helper()
	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
		"title": title, "points": points, "childIds": childIDs,
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("建指定范围积分项 = %d %s", rr.Code, rr.Body.String())
	}
	var it itemInfo
	json.Unmarshal(rr.Body.Bytes(), &it)
	return it
}

func TestRewardItemChildScope(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "scope01")
	pc := parentCookie(p.Token)
	a := mkChild(t, h, pc, "scope01a", "甲")
	b := mkChild(t, h, pc, "scope01b", "乙")

	var item itemResp
	// 省略 childIds → 非 nil 空切片（JSON 里是 []，不是 null）
	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{"title": "默认", "points": 5}, pc)
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.ChildIDs == nil || len(item.ChildIDs) != 0 {
		t.Fatalf("省略 childIds: %#v, want 非-nil 空切片", item.ChildIDs)
	}

	// 指定给甲
	rr = doJSON(h, "POST", "/api/reward-items", map[string]any{
		"title": "练琴", "points": 5, "childIds": []int64{a.ID},
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("create = %d %s", rr.Code, rr.Body.String())
	}
	json.Unmarshal(rr.Body.Bytes(), &item)
	if len(item.ChildIDs) != 1 || item.ChildIDs[0] != a.ID {
		t.Fatalf("create childIds = %v, want [%d]", item.ChildIDs, a.ID)
	}

	list := func() []itemResp {
		rr := doJSON(h, "GET", "/api/reward-items", nil, pc)
		var l struct {
			Items []itemResp `json:"items"`
		}
		json.Unmarshal(rr.Body.Bytes(), &l)
		return l.Items
	}
	byID := func(id int64) itemResp {
		for _, it := range list() {
			if it.ID == id {
				return it
			}
		}
		t.Fatalf("列表里没有项 %d", id)
		return itemResp{}
	}
	if got := byID(item.ID).ChildIDs; len(got) != 1 || got[0] != a.ID {
		t.Errorf("列表 childIds = %v, want [%d]", got, a.ID)
	}

	// PATCH 省略 / null → 不动
	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"points": 8}, pc)
	json.Unmarshal(rr.Body.Bytes(), &item)
	if item.Points != 8 || len(item.ChildIDs) != 1 || item.ChildIDs[0] != a.ID {
		t.Errorf("省略 childIds 的 PATCH 不应改范围: %+v", item)
	}
	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": nil}, pc)
	json.Unmarshal(rr.Body.Bytes(), &item)
	if rr.Code != 200 || len(item.ChildIDs) != 1 || item.ChildIDs[0] != a.ID {
		t.Errorf("childIds:null 应视为未传: code=%d %+v", rr.Code, item)
	}

	// 重复 id 去重
	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{
		"childIds": []int64{a.ID, b.ID, a.ID},
	}, pc)
	json.Unmarshal(rr.Body.Bytes(), &item)
	if len(item.ChildIDs) != 2 || item.ChildIDs[0] != a.ID || item.ChildIDs[1] != b.ID {
		t.Errorf("去重后 childIds = %v, want [%d %d]", item.ChildIDs, a.ID, b.ID)
	}

	// 非法 id → 422 child_not_found，且不半写
	other := regParent(t, svc, "scope01x")
	oc := mkChild(t, h, parentCookie(other.Token), "scope01xc", "丙")
	off := mkChild(t, h, pc, "scope01d", "丁")
	doJSON(h, "PATCH", apiPath("children", off.ID), map[string]any{"active": false}, pc)
	for name, bad := range map[string][]int64{
		"他家孩子": {oc.ID}, "家长本人": {p.UserID}, "不存在": {999999}, "已停用": {off.ID, a.ID},
	} {
		rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": bad}, pc)
		if rr.Code != 422 || errCode(t, rr) != "child_not_found" {
			t.Errorf("%s: = %d %s, want 422 child_not_found", name, rr.Code, rr.Body.String())
		}
	}
	if got := byID(item.ID).ChildIDs; len(got) != 2 {
		t.Errorf("非法 PATCH 后范围被改了: %v", got)
	}
	if rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
		"title": "坏", "points": 1, "childIds": []int64{999999},
	}, pc); rr.Code != 422 || errCode(t, rr) != "child_not_found" {
		t.Errorf("create 非法 childIds = %d %s, want 422 child_not_found", rr.Code, rr.Body.String())
	}
	for _, it := range list() {
		if it.Title == "坏" {
			t.Errorf("create 422 后半写出了项「坏」: %+v", it)
		}
	}

	// 跨家庭：B 家长带 childIds PATCH A 家的项 → 404
	if rr := doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": []int64{}}, parentCookie(other.Token)); rr.Code != 404 {
		t.Errorf("跨家庭 PATCH = %d, want 404", rr.Code)
	}

	// 软删除后列表仍带 childIds
	doJSON(h, "DELETE", apiPath("reward-items", item.ID), nil, pc)
	if got := byID(item.ID); got.Active || len(got.ChildIDs) != 2 {
		t.Errorf("软删除后列表项不符: %+v", got)
	}

	// [] → 所有孩子
	doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"active": true}, pc)
	rr = doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"childIds": []int64{}}, pc)
	json.Unmarshal(rr.Body.Bytes(), &item)
	if rr.Code != 200 || item.ChildIDs == nil || len(item.ChildIDs) != 0 {
		t.Errorf("childIds:[] 应改为所有孩子（非-nil 空切片）: code=%d %#v", rr.Code, item.ChildIDs)
	}
}

type prizeResp struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Emoji  string `json:"emoji"`
	Active bool   `json:"active"`
	Sort   int64  `json:"sort"`
}

func TestPrizeCRUD(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "parent02")
	pc := parentCookie(p.Token)

	// emoji 缺省 🎁
	rr := doJSON(h, "POST", "/api/prizes", map[string]any{"name": "水果糖1颗"}, pc)
	if rr.Code != 200 {
		t.Fatalf("create = %d %s", rr.Code, rr.Body.String())
	}
	var prize prizeResp
	json.Unmarshal(rr.Body.Bytes(), &prize)
	if prize.ID == 0 || prize.Name != "水果糖1颗" || prize.Emoji != "🎁" || !prize.Active {
		t.Errorf("create 响应不符: %+v", prize)
	}

	rr = doJSON(h, "PATCH", apiPath("prizes", prize.ID), map[string]any{
		"name": "巧克力1颗", "emoji": "🍫",
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("patch = %d", rr.Code)
	}
	json.Unmarshal(rr.Body.Bytes(), &prize)
	if prize.Name != "巧克力1颗" || prize.Emoji != "🍫" {
		t.Errorf("patch 后不符: %+v", prize)
	}

	rr = doJSON(h, "GET", "/api/prizes", nil, pc)
	var list struct {
		Prizes []prizeResp `json:"prizes"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Prizes) != 1 {
		t.Fatalf("奖品列表不符: %+v", list.Prizes)
	}

	rr = doJSON(h, "DELETE", apiPath("prizes", prize.ID), nil, pc)
	if rr.Code != 200 {
		t.Fatalf("delete = %d", rr.Code)
	}
}

func TestWheelConfigValidation(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "parent03")
	pc := parentCookie(p.Token)

	// 默认 5/7
	rr := doJSON(h, "GET", "/api/wheel-config", nil, pc)
	if rr.Code != 200 {
		t.Fatalf("get = %d", rr.Code)
	}
	var cfg struct {
		SingleCost int64 `json:"singleCost"`
		DoubleCost int64 `json:"doubleCost"`
	}
	json.Unmarshal(rr.Body.Bytes(), &cfg)
	if cfg.SingleCost != 5 || cfg.DoubleCost != 7 {
		t.Errorf("默认定价 = %d/%d, want 5/7", cfg.SingleCost, cfg.DoubleCost)
	}

	for _, bad := range []map[string]any{
		{"singleCost": 0, "doubleCost": 7},
		{"singleCost": 5, "doubleCost": 3},
		{"singleCost": 5, "doubleCost": 1000},
	} {
		rr = doJSON(h, "PUT", "/api/wheel-config", bad, pc)
		if rr.Code != 422 {
			t.Errorf("PUT %v = %d, want 422", bad, rr.Code)
		}
	}

	rr = doJSON(h, "PUT", "/api/wheel-config", map[string]any{
		"singleCost": 4, "doubleCost": 6,
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("put = %d %s", rr.Code, rr.Body.String())
	}
	fc := deps.Store.Family(p.FamilyID)
	if fc == nil || fc.SingleCost != 4 || fc.DoubleCost != 6 {
		t.Errorf("快照定价未更新: %+v", fc)
	}
}

func TestSnapshotRefresh(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	p := regParent(t, svc, "parent04")
	pc := parentCookie(p.Token)

	ids := make([]int64, 2)
	for i, title := range []string{"早读10分钟", "945上床睡觉"} {
		rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
			"title": title, "points": 5, "sort": i,
		}, pc)
		if rr.Code != 200 {
			t.Fatalf("create = %d", rr.Code)
		}
		var item itemResp
		json.Unmarshal(rr.Body.Bytes(), &item)
		ids[i] = item.ID
	}
	// 快照应含两个 active 项
	fc := deps.Store.Family(p.FamilyID)
	if fc == nil || len(fc.Items) != 2 {
		t.Fatalf("快照积分项 = %+v", fc)
	}

	// 停用一个 → 快照只剩一个
	rr := doJSON(h, "PATCH", apiPath("reward-items", ids[0]), map[string]any{"active": false}, pc)
	if rr.Code != 200 {
		t.Fatalf("停用 = %d", rr.Code)
	}
	fc = deps.Store.Family(p.FamilyID)
	if len(fc.Items) != 1 || fc.Items[0].Title != "945上床睡觉" {
		t.Errorf("停用后快照不符: %+v", fc.Items)
	}
}

func TestFamilyIsolation(t *testing.T) {
	deps, svc := newDeps(t)
	h := New(deps)
	pa := regParent(t, svc, "isopa")
	pb := regParent(t, svc, "isopb")

	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
		"title": "A项", "points": 5,
	}, parentCookie(pa.Token))
	var item itemResp
	json.Unmarshal(rr.Body.Bytes(), &item)

	// B 家操作 A 家资源 → 404
	if rr := doJSON(h, "PATCH", apiPath("reward-items", item.ID), map[string]any{"points": 9}, parentCookie(pb.Token)); rr.Code != 404 {
		t.Errorf("B PATCH A 项 = %d, want 404", rr.Code)
	}
	if rr := doJSON(h, "DELETE", apiPath("reward-items", item.ID), nil, parentCookie(pb.Token)); rr.Code != 404 {
		t.Errorf("B DELETE A 项 = %d, want 404", rr.Code)
	}
	// B 家列表为空
	rr = doJSON(h, "GET", "/api/reward-items", nil, parentCookie(pb.Token))
	var list struct {
		Items []itemResp `json:"items"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list.Items) != 0 {
		t.Errorf("B 家应无积分项: %d", len(list.Items))
	}
}
