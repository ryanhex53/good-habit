package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func loginAs(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	rr := doJSON(h, "POST", "/api/auth/login", map[string]any{
		"username": username, "password": password,
	})
	if rr.Code != 200 {
		t.Fatalf("login %s = %d %s", username, rr.Code, rr.Body.String())
	}
	c := sessionCookie(rr)
	if c == nil {
		t.Fatal("登录应下发 cookie")
	}
	return c
}

type drawSetupInfo struct {
	Parent    *http.Cookie
	Child     *http.Cookie
	ChildID   int64
	FamilyID  int64
}

// drawSetup 建好一家：2 个奖品、1 个 100 分积分项、孩子已登录并有 100 分。
func drawSetup(t *testing.T, uname string) (Deps, *wrapper, http.Handler, drawSetupInfo) {
	t.Helper()
	deps, w := newDepsWithWriter(t)
	h := New(deps)
	p := regParent(t, deps.Auth, uname)
	pc := parentCookie(p.Token)

	for _, pz := range []map[string]any{
		{"name": "水果糖1颗", "emoji": "🍬"},
		{"name": "巧克力1颗", "emoji": "🍫"},
	} {
		if rr := doJSON(h, "POST", "/api/prizes", pz, pc); rr.Code != 200 {
			t.Fatalf("建奖品 = %d %s", rr.Code, rr.Body.String())
		}
	}
	it := mkItem(t, h, pc, "大分项", 100)
	child := mkChild(t, h, pc, uname+"kid", "娃")
	if rr := doJSON(h, "POST", "/api/earnings", map[string]any{
		"childId": child.ID, "rewardItemId": it.ID,
	}, pc); rr.Code != 200 {
		t.Fatalf("earn = %d %s", rr.Code, rr.Body.String())
	}
	kc := loginAs(t, h, uname+"kid", "password8")
	return deps, &wrapper{w}, h, drawSetupInfo{Parent: pc, Child: kc, ChildID: child.ID, FamilyID: p.FamilyID}
}

// wrapper 让测试里 w.Stop() 的书写更短。
type wrapper struct{ w interface{ Stop() error } }

type prizeViewT struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

type drawRespT struct {
	Draw struct {
		UUID    string       `json:"uuid"`
		Mode    string       `json:"mode"`
		State   string       `json:"state"`
		Prize   *prizeViewT  `json:"prize"`
		Choices []prizeViewT `json:"choices"`
	} `json:"draw"`
	Entry struct {
		Delta        int64  `json:"delta"`
		BalanceAfter int64  `json:"balanceAfter"`
		Kind         string `json:"kind"`
	} `json:"entry"`
}

func TestSingleDrawFlow(t *testing.T) {
	deps, w, h, s := drawSetup(t, "drawer1")

	rr := doJSON(h, "POST", "/api/draws", map[string]any{"mode": "single"}, s.Child)
	if rr.Code != 200 {
		t.Fatalf("draw = %d %s", rr.Code, rr.Body.String())
	}
	var resp drawRespT
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Draw.UUID == "" || resp.Draw.Mode != "single" || resp.Draw.State != "completed" ||
		resp.Draw.Prize == nil || resp.Draw.Prize.Name == "" {
		t.Errorf("draw 响应不符: %+v", resp.Draw)
	}
	if resp.Entry.Delta != -5 || resp.Entry.BalanceAfter != 95 || resp.Entry.Kind != "draw" {
		t.Errorf("entry 不符: %+v", resp.Entry)
	}

	bal, pending, unful := deps.Store.Child(s.ChildID).Stats()
	if bal != 95 || pending != "" || unful != 1 {
		t.Errorf("内存态 = %d/%q/%d, want 95/\"\"/1", bal, pending, unful)
	}

	// 落盘校验
	if err := w.w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	var drawID, picked int64
	var state string
	var fulfilled *int64
	err := deps.DBs.Read.QueryRow(
		"SELECT id, picked, state, fulfilled_at FROM draws WHERE uuid=?", resp.Draw.UUID,
	).Scan(&drawID, &picked, &state, &fulfilled)
	if err != nil || state != "completed" || picked != 0 || fulfilled != nil {
		t.Fatalf("draws 行不符: id=%d picked=%d state=%s err=%v", drawID, picked, state, err)
	}
	var delta, balAfter, refDraw int64
	var kind string
	err = deps.DBs.Read.QueryRow(
		"SELECT delta, balance_after, kind, ref_draw_id FROM ledger WHERE child_id=? AND kind='draw'",
		s.ChildID,
	).Scan(&delta, &balAfter, &kind, &refDraw)
	if err != nil || delta != -5 || balAfter != 95 || kind != "draw" || refDraw != drawID {
		t.Fatalf("ledger 行不符: %d/%d/%s/%d err=%v", delta, balAfter, kind, refDraw, err)
	}

	// 发放
	rr = doJSON(h, "PATCH", apiPath("draws", resp.Draw.UUID)+"/fulfill", nil, s.Parent)
	if rr.Code != 200 {
		t.Fatalf("fulfill = %d %s", rr.Code, rr.Body.String())
	}
	_, _, unful = deps.Store.Child(s.ChildID).Stats()
	if unful != 0 {
		t.Errorf("发放后 Unfulfilled = %d, want 0", unful)
	}
	var fulfilledAt int64
	deps.DBs.Read.QueryRow(
		"SELECT fulfilled_at FROM draws WHERE uuid=?", resp.Draw.UUID).Scan(&fulfilledAt)
	if fulfilledAt == 0 {
		t.Error("fulfilled_at 应非零")
	}
	rr = doJSON(h, "PATCH", apiPath("draws", resp.Draw.UUID)+"/fulfill", nil, s.Parent)
	if rr.Code != 409 || errCode(t, rr) != "already_fulfilled" {
		t.Errorf("重复发放 = %d %s, want 409 already_fulfilled", rr.Code, rr.Body.String())
	}
}

func TestDoubleDrawFlow(t *testing.T) {
	deps, w, h, s := drawSetup(t, "drawer2")

	rr := doJSON(h, "POST", "/api/draws", map[string]any{"mode": "double"}, s.Child)
	if rr.Code != 200 {
		t.Fatalf("draw = %d %s", rr.Code, rr.Body.String())
	}
	var resp drawRespT
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Draw.State != "pending_choice" || len(resp.Draw.Choices) != 2 {
		t.Fatalf("双抽响应不符: %+v", resp.Draw)
	}
	if resp.Entry.Delta != -7 || resp.Entry.BalanceAfter != 93 {
		t.Errorf("entry 不符: %+v", resp.Entry)
	}

	bal, pending, _ := deps.Store.Child(s.ChildID).Stats()
	if bal != 93 || pending != resp.Draw.UUID {
		t.Errorf("内存态 = %d/%q, want 93/%s", bal, pending, resp.Draw.UUID)
	}

	// 有未选局再抽 → 409
	rr = doJSON(h, "POST", "/api/draws", map[string]any{"mode": "single"}, s.Child)
	if rr.Code != 409 || errCode(t, rr) != "pending_choice_exists" {
		t.Fatalf("待选局再抽 = %d %s, want 409 pending_choice_exists", rr.Code, rr.Body.String())
	}

	// 非法 pick → 422
	rr = doJSON(h, "POST", apiPath("draws", resp.Draw.UUID)+"/choose", map[string]any{"pick": 5}, s.Child)
	if rr.Code != 422 {
		t.Fatalf("非法 pick = %d, want 422", rr.Code)
	}

	// 选择 1
	rr = doJSON(h, "POST", apiPath("draws", resp.Draw.UUID)+"/choose", map[string]any{"pick": 1}, s.Child)
	if rr.Code != 200 {
		t.Fatalf("choose = %d %s", rr.Code, rr.Body.String())
	}
	var chosen drawRespT
	json.Unmarshal(rr.Body.Bytes(), &chosen)
	if chosen.Draw.Prize == nil || chosen.Draw.Prize.Name != resp.Draw.Choices[1].Name {
		t.Errorf("选中奖品不符: %+v want %+v", chosen.Draw.Prize, resp.Draw.Choices[1])
	}

	bal, pending, unful := deps.Store.Child(s.ChildID).Stats()
	if bal != 93 || pending != "" || unful != 1 {
		t.Errorf("选择后内存态 = %d/%q/%d, want 93/\"\"/1", bal, pending, unful)
	}

	// 重复选择 → 409
	rr = doJSON(h, "POST", apiPath("draws", resp.Draw.UUID)+"/choose", map[string]any{"pick": 0}, s.Child)
	if rr.Code != 409 {
		t.Fatalf("重复选择 = %d, want 409", rr.Code)
	}

	if err := w.w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	var picked int64
	var state string
	deps.DBs.Read.QueryRow(
		"SELECT picked, state FROM draws WHERE uuid=?", resp.Draw.UUID).Scan(&picked, &state)
	if picked != 1 || state != "completed" {
		t.Errorf("落盘 = picked %d state %s, want 1/completed", picked, state)
	}
}

func TestEmptyPrizePool(t *testing.T) {
	deps, _, h, s := drawSetup(t, "drawer3")
	// 停用全部奖品
	rr := doJSON(h, "GET", "/api/prizes", nil, s.Parent)
	var list struct {
		Prizes []struct{ ID int64 `json:"id"` } `json:"prizes"`
	}
	json.Unmarshal(rr.Body.Bytes(), &list)
	for _, pz := range list.Prizes {
		doJSON(h, "PATCH", apiPath("prizes", pz.ID), map[string]any{"active": false}, s.Parent)
	}

	rr = doJSON(h, "POST", "/api/draws", map[string]any{"mode": "single"}, s.Child)
	if rr.Code != 409 || errCode(t, rr) != "no_active_prizes" {
		t.Fatalf("空池 = %d %s, want 409 no_active_prizes", rr.Code, rr.Body.String())
	}
	bal, _, _ := deps.Store.Child(s.ChildID).Stats()
	if bal != 100 {
		t.Errorf("空池扣费 = %d, want 100 不变", bal)
	}
	var n int64
	deps.DBs.Read.QueryRow("SELECT COUNT(*) FROM draws WHERE child_id=?", s.ChildID).Scan(&n)
	if n != 0 {
		t.Errorf("draws 行 = %d, want 0", n)
	}
}

func TestInsufficientBalance(t *testing.T) {
	deps, _, h, s := drawSetup(t, "drawer4")
	// 把余额调到 3
	doJSON(h, "POST", "/api/adjustments", map[string]any{
		"childId": s.ChildID, "delta": -97, "note": "清零",
	}, s.Parent)

	rr := doJSON(h, "POST", "/api/draws", map[string]any{"mode": "single"}, s.Child)
	if rr.Code != 409 || errCode(t, rr) != "insufficient_points" {
		t.Fatalf("余额不足 = %d %s, want 409 insufficient_points", rr.Code, rr.Body.String())
	}
	bal, pending, unful := deps.Store.Child(s.ChildID).Stats()
	if bal != 3 || pending != "" || unful != 0 {
		t.Errorf("零副作用检查 = %d/%q/%d, want 3/\"\"/0", bal, pending, unful)
	}

	// 坏 mode → 422
	rr = doJSON(h, "POST", "/api/draws", map[string]any{"mode": "triple"}, s.Child)
	if rr.Code != 422 {
		t.Errorf("坏 mode = %d, want 422", rr.Code)
	}
}

func TestConcurrentDraws(t *testing.T) {
	deps, w, h, s := drawSetup(t, "drawer5")
	// 余额调到 25 = 恰好 5 次单抽
	doJSON(h, "POST", "/api/adjustments", map[string]any{
		"childId": s.ChildID, "delta": -75, "note": "调到 25",
	}, s.Parent)

	var wg sync.WaitGroup
	var ok200, fail409 int64
	var mu sync.Mutex
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/draws", jsonReader(map[string]any{"mode": "single"}))
			req.AddCookie(s.Child)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			mu.Lock()
			defer mu.Unlock()
			if rr.Code == 200 {
				ok200++
			} else if rr.Code == 409 {
				fail409++
			}
		}()
	}
	wg.Wait()

	if ok200 != 5 || fail409 != 5 {
		t.Errorf("并发结果 = %d 成功 / %d 拒绝, want 5/5", ok200, fail409)
	}
	if bal, _, _ := deps.Store.Child(s.ChildID).Stats(); bal != 0 {
		t.Errorf("终余额 = %d, want 0", bal)
	}
	if err := w.w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	var cnt, sum int64
	deps.DBs.Read.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(delta),0) FROM ledger WHERE child_id=? AND kind='draw'",
		s.ChildID).Scan(&cnt, &sum)
	if cnt != 5 || sum != -25 {
		t.Errorf("落盘 = COUNT %d SUM %d, want 5/-25", cnt, sum)
	}
}

func TestChooseForeignDraw(t *testing.T) {
	deps, _, h, s := drawSetup(t, "drawer6")

	// B 家孩子
	pb := regParent(t, deps.Auth, "drawer6b")
	pbc := parentCookie(pb.Token)
	cb := mkChild(t, h, pbc, "drawer6bkid", "B娃")
	doJSON(h, "POST", "/api/earnings", map[string]any{
		"childId": cb.ID, "rewardItemId": mkItem(t, h, pbc, "B项", 50).ID,
	}, pbc)
	kb := loginAs(t, h, "drawer6bkid", "password8")

	// A 家孩子开一局双抽
	rr := doJSON(h, "POST", "/api/draws", map[string]any{"mode": "double"}, s.Child)
	var resp drawRespT
	json.Unmarshal(rr.Body.Bytes(), &resp)

	// B 家孩子选 A 家的局 → 404
	rr = doJSON(h, "POST", apiPath("draws", resp.Draw.UUID)+"/choose", map[string]any{"pick": 0}, kb)
	if rr.Code != 404 {
		t.Errorf("跨家庭 choose = %d, want 404", rr.Code)
	}
}
