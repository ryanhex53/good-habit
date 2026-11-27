package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
)

type ledgerRowT struct {
	ID           int64  `json:"id"`
	CreatedAt    int64  `json:"createdAt"`
	Kind         string `json:"kind"`
	Delta        int64  `json:"delta"`
	BalanceAfter int64  `json:"balanceAfter"`
	ItemTitle    string `json:"itemTitle"`
	PrizeName    string `json:"prizeName"`
	Mode         string `json:"mode"`
	Note         string `json:"note"`
	ChildName    string `json:"childName"`
}

func TestLedgerPagination(t *testing.T) {
	deps, _, h, s := drawSetup(t, "pager1")
	// 直接铺 35 行账单夹具（纯分页测试不需要走业务路径）
	now := int64(1730000000000)
	err := deps.DBs.WithTx(func(tx *sql.Tx) error {
		for i := 0; i < 35; i++ {
			if _, err := tx.Exec(
				`INSERT INTO ledger (uuid, family_id, child_id, delta, balance_after, kind, created_at)
				 VALUES (?,?,?,?,?,'earn',?)`,
				fmt.Sprintf("page-%d-%d", s.ChildID, i), s.FamilyID, s.ChildID,
				1, int64(i+1), now+int64(i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("铺夹具: %v", err)
	}

	rr := doJSON(h, "GET", "/api/ledger?limit=30", nil, s.Child)
	if rr.Code != 200 {
		t.Fatalf("第一页 = %d %s", rr.Code, rr.Body.String())
	}
	var page1 struct {
		Rows       []ledgerRowT `json:"rows"`
		NextBefore *int64       `json:"nextBefore"`
	}
	json.Unmarshal(rr.Body.Bytes(), &page1)
	if len(page1.Rows) != 30 {
		t.Fatalf("第一页行数 = %d, want 30", len(page1.Rows))
	}
	if page1.NextBefore == nil || *page1.NextBefore != page1.Rows[29].ID {
		t.Fatalf("nextBefore = %v, want 第 30 行 id %d", page1.NextBefore, page1.Rows[29].ID)
	}
	// id 倒序
	if page1.Rows[0].ID <= page1.Rows[1].ID {
		t.Error("应按 id 倒序")
	}

	rr = doJSON(h, "GET", fmt.Sprintf("/api/ledger?limit=30&before=%d", *page1.NextBefore), nil, s.Child)
	var page2 struct {
		Rows       []ledgerRowT `json:"rows"`
		NextBefore *int64       `json:"nextBefore"`
	}
	json.Unmarshal(rr.Body.Bytes(), &page2)
	// 35 夹具行 + drawSetup 的 1 条 earn = 36 行；第二页 36-30 = 6
	if len(page2.Rows) != 6 {
		t.Fatalf("第二页行数 = %d, want 6", len(page2.Rows))
	}
	if page2.NextBefore != nil {
		t.Errorf("尾页 nextBefore 应为 null，得到 %v", *page2.NextBefore)
	}
}

func TestLedgerDisplayJoin(t *testing.T) {
	_, w, h, s := drawSetup(t, "pager2")

	rr := doJSON(h, "POST", "/api/draws", map[string]any{"mode": "single"}, s.Child)
	var resp drawRespT
	json.Unmarshal(rr.Body.Bytes(), &resp)
	doJSON(h, "POST", "/api/adjustments", map[string]any{
		"childId": s.ChildID, "delta": -20, "note": "测试调整",
	}, s.Parent)
	if err := w.w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	rr = doJSON(h, "GET", "/api/ledger", nil, s.Child)
	if rr.Code != 200 {
		t.Fatalf("ledger = %d %s", rr.Code, rr.Body.String())
	}
	var page struct {
		Rows []ledgerRowT `json:"rows"`
	}
	json.Unmarshal(rr.Body.Bytes(), &page)
	if len(page.Rows) != 3 {
		t.Fatalf("行数 = %d, want 3（adjust/draw/earn）", len(page.Rows))
	}
	var drawRow, earnRow, adjustRow *ledgerRowT
	for i := range page.Rows {
		switch page.Rows[i].Kind {
		case "draw":
			drawRow = &page.Rows[i]
		case "earn":
			earnRow = &page.Rows[i]
		case "adjust":
			adjustRow = &page.Rows[i]
		}
	}
	if drawRow == nil || drawRow.PrizeName != resp.Draw.Prize.Name || drawRow.Mode != "single" ||
		drawRow.Delta != -5 || drawRow.BalanceAfter != 95 {
		t.Errorf("draw 行不符: %+v (want prize=%s mode=single delta=-5 bal=95)",
			drawRow, resp.Draw.Prize.Name)
	}
	if earnRow == nil || earnRow.ItemTitle != "大分项" || earnRow.Delta != 100 {
		t.Errorf("earn 行不符: %+v", earnRow)
	}
	if adjustRow == nil || adjustRow.Note != "测试调整" || adjustRow.Delta != -20 {
		t.Errorf("adjust 行不符: %+v", adjustRow)
	}
	// 每种 kind 的行都带孩子名字（孩子自己查也一样）
	for _, r := range []*ledgerRowT{drawRow, earnRow, adjustRow} {
		if r != nil && r.ChildName != "娃" {
			t.Errorf("%s 行 childName = %q, want 娃", r.Kind, r.ChildName)
		}
	}

	// 家长带 childId 查询：能看到同样内容
	rr = doJSON(h, "GET", fmt.Sprintf("/api/ledger?childId=%d", s.ChildID), nil, s.Parent)
	var parentPage struct {
		Rows []ledgerRowT `json:"rows"`
	}
	json.Unmarshal(rr.Body.Bytes(), &parentPage)
	if len(parentPage.Rows) != len(page.Rows) {
		t.Errorf("家长按 childId 查 = %d 行, want %d", len(parentPage.Rows), len(page.Rows))
	}
}

// 每行的 childName 必须来自该行自己的 child_id（JOIN 按行取），不是一个常量。
func TestLedgerChildNamePerRow(t *testing.T) {
	_, w, h, s := drawSetup(t, "ledgername") // 已有孩子「娃」的一笔 earn
	it := mkItem(t, h, s.Parent, "小项", 1)
	jia := mkChild(t, h, s.Parent, "ledgernamejia", "甲")
	yi := mkChild(t, h, s.Parent, "ledgernameyi", "乙")
	for _, c := range []childInfo{jia, yi} {
		if rr := doJSON(h, "POST", "/api/earnings", map[string]any{
			"childId": c.ID, "rewardItemId": it.ID,
		}, s.Parent); rr.Code != 200 {
			t.Fatalf("earn = %d %s", rr.Code, rr.Body.String())
		}
	}
	if err := w.w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	rr := doJSON(h, "GET", "/api/ledger", nil, s.Parent)
	if rr.Code != 200 {
		t.Fatalf("ledger = %d %s", rr.Code, rr.Body.String())
	}
	var page struct {
		Rows []ledgerRowT `json:"rows"`
	}
	json.Unmarshal(rr.Body.Bytes(), &page)
	if len(page.Rows) != 3 {
		t.Fatalf("行数 = %d, want 3", len(page.Rows))
	}
	want := []string{"乙", "甲", "娃"} // id DESC：乙最后登记排最前
	for i, name := range want {
		if page.Rows[i].ChildName != name {
			t.Errorf("rows[%d].childName = %q, want %q", i, page.Rows[i].ChildName, name)
		}
	}

	rr = doJSON(h, "GET", fmt.Sprintf("/api/ledger?childId=%d", jia.ID), nil, s.Parent)
	json.Unmarshal(rr.Body.Bytes(), &page)
	if len(page.Rows) != 1 || page.Rows[0].ChildName != "甲" {
		t.Errorf("按 childId=甲 查 = %+v, want 1 行且 childName=甲", page.Rows)
	}
}

func TestLedgerIsolation(t *testing.T) {
	deps, _, h, _ := drawSetup(t, "pager3")

	// B 家孩子
	pb := regParent(t, deps.Auth, "pager3b")
	pbc := parentCookie(pb.Token)
	cb := mkChild(t, h, pbc, "pager3bkid", "B娃")
	kb := loginAs(t, h, "pager3bkid", "password8")

	// A 家孩子有一条 earn（drawSetup 已给），B 家孩子查 → 空
	rr := doJSON(h, "GET", "/api/ledger", nil, kb)
	if rr.Code != 200 {
		t.Fatalf("B 孩子 ledger = %d", rr.Code)
	}
	var page struct {
		Rows []ledgerRowT `json:"rows"`
	}
	json.Unmarshal(rr.Body.Bytes(), &page)
	if len(page.Rows) != 0 {
		t.Errorf("B 家孩子应查不到 A 家账单: %d 行", len(page.Rows))
	}
	_ = cb
}

func TestWheelAndGuide(t *testing.T) {
	_, _, h, s := drawSetup(t, "pager4")

	// 开一局双抽形成 pending
	rr := doJSON(h, "POST", "/api/draws", map[string]any{"mode": "double"}, s.Child)
	var resp drawRespT
	json.Unmarshal(rr.Body.Bytes(), &resp)

	rr = doJSON(h, "GET", "/api/wheel", nil, s.Child)
	if rr.Code != 200 {
		t.Fatalf("wheel = %d %s", rr.Code, rr.Body.String())
	}
	var wheel struct {
		Prizes     []prizeViewT `json:"prizes"`
		SingleCost int64        `json:"singleCost"`
		DoubleCost int64        `json:"doubleCost"`
		Balance    int64        `json:"balance"`
		Pending    *struct {
			UUID     string       `json:"uuid"`
			Choices  []prizeViewT `json:"choices"`
			Deadline int64        `json:"deadline"`
		} `json:"pending"`
	}
	json.Unmarshal(rr.Body.Bytes(), &wheel)
	if len(wheel.Prizes) != 2 || wheel.SingleCost != 5 || wheel.DoubleCost != 7 || wheel.Balance != 93 {
		t.Errorf("wheel 基本字段不符: %+v", wheel)
	}
	if wheel.Pending == nil || wheel.Pending.UUID != resp.Draw.UUID ||
		len(wheel.Pending.Choices) != 2 || wheel.Pending.Deadline == 0 {
		t.Errorf("pending 不符: %+v", wheel.Pending)
	}

	rr = doJSON(h, "GET", "/api/guide", nil, s.Child)
	if rr.Code != 200 {
		t.Fatalf("guide = %d", rr.Code)
	}
	var guide struct {
		Items []struct {
			ID     int64  `json:"id"`
			Title  string `json:"title"`
			Points int64  `json:"points"`
		} `json:"items"`
	}
	json.Unmarshal(rr.Body.Bytes(), &guide)
	if len(guide.Items) != 1 || guide.Items[0].Title != "大分项" || guide.Items[0].Points != 100 {
		t.Errorf("guide 不符: %+v", guide.Items)
	}

	// 家长不能访问 child 端点
	rr = doJSON(h, "GET", "/api/wheel", nil, s.Parent)
	if rr.Code != 403 {
		t.Errorf("家长访问 wheel = %d, want 403", rr.Code)
	}
}

func TestOverview(t *testing.T) {
	_, _, h, s := drawSetup(t, "pager5")

	rr := doJSON(h, "GET", "/api/overview", nil, s.Parent)
	if rr.Code != 200 {
		t.Fatalf("overview = %d %s", rr.Code, rr.Body.String())
	}
	var ov struct {
		Children []struct {
			ID          int64  `json:"id"`
			DisplayName string `json:"displayName"`
			Balance     int64  `json:"balance"`
			Unfulfilled int64  `json:"unfulfilled"`
			HasPending  bool   `json:"hasPending"`
		} `json:"children"`
		TodayEarns int64 `json:"todayEarns"`
	}
	json.Unmarshal(rr.Body.Bytes(), &ov)
	if len(ov.Children) != 1 || ov.Children[0].ID != s.ChildID ||
		ov.Children[0].Balance != 100 || ov.Children[0].Unfulfilled != 0 {
		t.Fatalf("overview 不符: %+v", ov.Children)
	}
	if ov.TodayEarns != 1 {
		t.Errorf("todayEarns = %d, want 1（drawSetup 有一次 earn）", ov.TodayEarns)
	}

	// 抽一次后余额/待发放变化
	doJSON(h, "POST", "/api/draws", map[string]any{"mode": "single"}, s.Child)
	rr = doJSON(h, "GET", "/api/overview", nil, s.Parent)
	json.Unmarshal(rr.Body.Bytes(), &ov)
	if ov.Children[0].Balance != 95 || ov.Children[0].Unfulfilled != 1 {
		t.Errorf("抽奖后 overview 不符: %+v", ov.Children[0])
	}
}

func TestGuideScopedByChild(t *testing.T) {
	deps, _ := newDepsWithWriter(t)
	h := New(deps)
	p := regParent(t, deps.Auth, "guide1")
	pc := parentCookie(p.Token)
	a := mkChild(t, h, pc, "guide1a", "甲")
	b := mkChild(t, h, pc, "guide1b", "乙")

	mkItem(t, h, pc, "大家的", 1)
	mkScopedItem(t, h, pc, "只给甲", 2, a.ID)
	mkScopedItem(t, h, pc, "只给乙", 3, b.ID)

	titles := func(cookie *http.Cookie) []string {
		rr := doJSON(h, "GET", "/api/guide", nil, cookie)
		if rr.Code != 200 {
			t.Fatalf("guide = %d %s", rr.Code, rr.Body.String())
		}
		var g struct {
			Items []struct {
				Title string `json:"title"`
			} `json:"items"`
		}
		json.Unmarshal(rr.Body.Bytes(), &g)
		out := []string{}
		for _, it := range g.Items {
			out = append(out, it.Title)
		}
		return out
	}
	ka := loginAs(t, h, "guide1a", "password8")
	kb := loginAs(t, h, "guide1b", "password8")
	if got, want := titles(ka), []string{"大家的", "只给甲"}; !slices.Equal(got, want) {
		t.Errorf("甲的指南 = %v, want %v", got, want)
	}
	if got, want := titles(kb), []string{"大家的", "只给乙"}; !slices.Equal(got, want) {
		t.Errorf("乙的指南 = %v, want %v", got, want)
	}
}
