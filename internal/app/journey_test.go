package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"good-habit/internal/config"
)

func TestFullJourney(t *testing.T) {
	cfg := config.Config{
		Addr: "127.0.0.1:0", DBPath: filepath.Join(t.TempDir(), "journey.db"), ReadPool: 5,
	}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := a.Handler()

	// ---------- 阶段一：注册 → 种子 → 孩子 → 得分 → 抽奖 ----------
	rr := postJSON(h, "/api/auth/register", map[string]any{
		"username": "journey1", "password": "password8", "family_name": "旅程家",
	})
	if rr.Code != 200 {
		t.Fatalf("register = %d %s", rr.Code, rr.Body.String())
	}
	pc := cookieOf(rr)

	rr = postJSON(h, "/api/bootstrap", map[string]any{}, pc)
	if rr.Code != 200 {
		t.Fatalf("bootstrap = %d %s", rr.Code, rr.Body.String())
	}

	// 建两个孩子
	mkKid := func(uname, disp string) int64 {
		rr = postJSON(h, "/api/children", map[string]any{
			"username": uname, "password": "password8", "displayName": disp,
		}, pc)
		if rr.Code != 200 {
			t.Fatalf("建孩子 = %d %s", rr.Code, rr.Body.String())
		}
		var c struct{ ID int64 `json:"id"` }
		json.Unmarshal(rr.Body.Bytes(), &c)
		return c.ID
	}
	k1 := mkKid("journeykid1", "大娃")
	k2 := mkKid("journeykid2", "二娃")

	kc := loginCookie(t, h, "journeykid1", "password8")
	kc2 := loginCookie(t, h, "journeykid2", "password8")

	// 取前三个积分项 id
	rr = getJSON(h, "/api/reward-items", pc)
	var items struct {
		Items []struct{ ID int64 `json:"id"` } `json:"items"`
	}
	json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items.Items) < 3 {
		t.Fatalf("bootstrap 后积分项 = %d, want ≥11", len(items.Items))
	}

	earn := func(cookie *http.Cookie, childID, itemID int64) {
		rr = postJSON(h, "/api/earnings", map[string]any{
			"childId": childID, "rewardItemId": itemID,
		}, cookie)
		if rr.Code != 200 {
			t.Fatalf("earn = %d %s", rr.Code, rr.Body.String())
		}
	}
	earn(pc, k1, items.Items[0].ID) // +7
	earn(pc, k1, items.Items[1].ID) // +5
	earn(pc, k1, items.Items[2].ID) // +10 → 22

	// 单抽（-5 → 17）
	var single struct {
		Draw struct{ UUID string `json:"uuid"` } `json:"draw"`
	}
	rr = postJSON(h, "/api/draws", map[string]any{"mode": "single"}, kc)
	if rr.Code != 200 {
		t.Fatalf("单抽 = %d %s", rr.Code, rr.Body.String())
	}
	json.Unmarshal(rr.Body.Bytes(), &single)

	// 双抽（-7 → 10），保持 pending 跨重启
	var dbl struct {
		Draw struct{ UUID string `json:"uuid"` } `json:"draw"`
	}
	rr = postJSON(h, "/api/draws", map[string]any{"mode": "double"}, kc)
	if rr.Code != 200 {
		t.Fatalf("双抽 = %d %s", rr.Code, rr.Body.String())
	}
	json.Unmarshal(rr.Body.Bytes(), &dbl)

	// 手动调整 +20（→ 30）；二娃 earn 一次（+5）
	rr = postJSON(h, "/api/adjustments", map[string]any{
		"childId": k1, "delta": 20, "note": "奖励补记",
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("adjust = %d %s", rr.Code, rr.Body.String())
	}
	earn(pc, k2, items.Items[0].ID)

	// 总览（内存直读）
	rr = getJSON(h, "/api/overview", pc)
	var ov struct {
		Children []struct {
			ID         int64  `json:"id"`
			Balance    int64  `json:"balance"`
			Unfulfilled int64 `json:"unfulfilled"`
			HasPending bool   `json:"hasPending"`
		} `json:"children"`
		TodayEarns int64 `json:"todayEarns"`
	}
	json.Unmarshal(rr.Body.Bytes(), &ov)
	if len(ov.Children) != 2 {
		t.Fatalf("overview 孩子 = %d", len(ov.Children))
	}
	if ov.Children[0].ID != k1 || ov.Children[0].Balance != 30 ||
		ov.Children[0].Unfulfilled != 1 || !ov.Children[0].HasPending {
		t.Errorf("大娃 overview 不符: %+v", ov.Children[0])
	}
	if ov.Children[1].ID != k2 || ov.Children[1].Balance != 7 || ov.Children[1].Unfulfilled != 0 {
		t.Errorf("二娃 overview 不符: %+v", ov.Children[1])
	}
	if ov.TodayEarns != 4 {
		t.Errorf("todayEarns = %d, want 4", ov.TodayEarns)
	}

	// 账单分页翻完无缺行（大娃 6 行：3 earn + 2 draw + 1 adjust）；
	// 先 Flush 排空组提交窗口，模拟用户翻页时数据已落盘
	if err := a.writer.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// 账单分页翻完无缺行（大娃 6 行：3 earn + 2 draw + 1 adjust）
	total := 0
	before := ""
	for {
		url := "/api/ledger?limit=3" + func() string {
			if before != "" {
				return "&before=" + before
			}
			return ""
		}()
		rr = getJSON(h, url, kc)
		if rr.Code != 200 {
			t.Fatalf("ledger = %d", rr.Code)
		}
		var page struct {
			Rows []struct {
				ID int64 `json:"id"`
			} `json:"rows"`
			NextBefore *int64 `json:"nextBefore"`
		}
		json.Unmarshal(rr.Body.Bytes(), &page)
		total += len(page.Rows)
		if page.NextBefore == nil {
			break
		}
		before = int64Str(*page.NextBefore)
	}
	if total != 6 {
		t.Errorf("账单总行数 = %d, want 6", total)
	}

	memBalK1 := a.store.Child(k1).Bal()
	if memBalK1 != 30 {
		t.Fatalf("停机前内存余额 = %d, want 30", memBalK1)
	}

	// ---------- 优雅停机 ----------
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// ---------- 不变量核对（冻结的 DB） ----------
	probe, err := openProbe(cfg.DBPath)
	if err != nil {
		t.Fatalf("openProbe: %v", err)
	}
	defer probe.Close()

	// 1. 账单余额链
	rows, err := probe.Read.Query(
		"SELECT delta, balance_after FROM ledger WHERE child_id=? ORDER BY id", k1)
	if err != nil {
		t.Fatalf("查链: %v", err)
	}
	var prev int64
	i := 0
	wantChain := [][2]int64{{7, 7}, {5, 12}, {10, 22}, {-5, 17}, {-7, 10}, {20, 30}}
	for rows.Next() {
		var d, b int64
		if err := rows.Scan(&d, &b); err != nil {
			rows.Close()
			t.Fatalf("scan 链: %v", err)
		}
		if i >= len(wantChain) {
			t.Fatalf("链行数超出: %d", i+1)
		}
		if d != wantChain[i][0] || b != wantChain[i][1] || prev+d != b {
			t.Errorf("第 %d 行 (delta=%d bal=%d) 不自洽（前值 %d）", i, d, b, prev)
		}
		prev = b
		i++
	}
	rows.Close()
	if i != len(wantChain) {
		t.Errorf("链行数 = %d, want %d", i, len(wantChain))
	}

	// 2. SUM == users.balance == 内存余额
	var sum, dbBal int64
	probe.Read.QueryRow("SELECT COALESCE(SUM(delta),0) FROM ledger WHERE child_id=?", k1).Scan(&sum)
	probe.Read.QueryRow("SELECT balance FROM users WHERE id=?", k1).Scan(&dbBal)
	if sum != 30 || dbBal != 30 || memBalK1 != 30 {
		t.Errorf("SUM/DB/内存 = %d/%d/%d, want 30/30/30", sum, dbBal, memBalK1)
	}

	// 3. 每个已完成/挂起的 draw 恰有一条 -cost 账单行
	drows, err := probe.Read.Query(
		`SELECT d.cost, (SELECT COUNT(*) FROM ledger l WHERE l.ref_draw_id = d.id AND l.delta = -d.cost)
		 FROM draws d WHERE d.child_id=?`, k1)
	if err != nil {
		t.Fatalf("查 draw 对账: %v", err)
	}
	nDraws := 0
	for drows.Next() {
		var cost, n int64
		if err := drows.Scan(&cost, &n); err != nil {
			drows.Close()
			t.Fatalf("scan draw: %v", err)
		}
		if n != 1 {
			t.Errorf("cost=%d 的 draw 有 %d 条 -cost 账单行, want 1", cost, n)
		}
		nDraws++
	}
	drows.Close()
	if nDraws != 2 {
		t.Errorf("draw 行数 = %d, want 2", nDraws)
	}

	// ---------- 阶段二：重启恢复 ----------
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

	// 会话与余额恢复
	if rr := getJSON(h2, "/api/me", kc); rr.Code != 200 {
		t.Errorf("重启后大娃 me = %d, want 200", rr.Code)
	}
	if bal := a2.store.Child(k1).Bal(); bal != 30 {
		t.Errorf("重启后大娃余额 = %d, want 30", bal)
	}

	// 双抽 pending 仍在
	rr = getJSON(h2, "/api/wheel", kc)
	var wheel struct {
		Balance int64 `json:"balance"`
		Pending *struct {
			UUID string `json:"uuid"`
		} `json:"pending"`
	}
	json.Unmarshal(rr.Body.Bytes(), &wheel)
	if wheel.Balance != 30 || wheel.Pending == nil || wheel.Pending.UUID != dbl.Draw.UUID {
		t.Errorf("重启后 wheel = bal %d pending %+v, want 30/%s", wheel.Balance, wheel.Pending, dbl.Draw.UUID)
	}

	// 重启后选完 + 发放
	rr = postJSON(h2, "/api/draws/"+dbl.Draw.UUID+"/choose", map[string]any{"pick": 0}, kc)
	if rr.Code != 200 {
		t.Fatalf("重启后 choose = %d %s", rr.Code, rr.Body.String())
	}
	rr = patchJSON(h2, "/api/draws/"+dbl.Draw.UUID+"/fulfill", pc)
	if rr.Code != 200 {
		t.Fatalf("发放 = %d %s", rr.Code, rr.Body.String())
	}
	// 单抽的发放
	rr = patchJSON(h2, "/api/draws/"+single.Draw.UUID+"/fulfill", pc)
	if rr.Code != 200 {
		t.Fatalf("单抽发放 = %d %s", rr.Code, rr.Body.String())
	}

	// 终态总览：unfulfilled 0、无 pending
	rr = getJSON(h2, "/api/overview", pc)
	json.Unmarshal(rr.Body.Bytes(), &ov)
	if ov.Children[0].Unfulfilled != 0 || ov.Children[0].HasPending {
		t.Errorf("终态大娃 overview 不符: %+v", ov.Children[0])
	}

	// 二娃无变化
	if rr := getJSON(h2, "/api/me", kc2); rr.Code != 200 {
		t.Errorf("重启后二娃 me = %d, want 200", rr.Code)
	}
}

func loginCookie(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	rr := postJSON(h, "/api/auth/login", map[string]any{
		"username": username, "password": password,
	})
	if rr.Code != 200 {
		t.Fatalf("login %s = %d %s", username, rr.Code, rr.Body.String())
	}
	c := cookieOf(rr)
	if c == nil {
		t.Fatal("登录应下发 cookie")
	}
	return c
}

func patchJSON(h http.Handler, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PATCH", path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func int64Str(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
