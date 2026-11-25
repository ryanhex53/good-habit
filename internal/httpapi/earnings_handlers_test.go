package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"good-habit/internal/auth"
	"good-habit/internal/config"
	"good-habit/internal/db"
	"good-habit/internal/ledger"
	"good-habit/internal/store"
	"good-habit/internal/writer"
	"good-habit/migrations"
)

// jsonReader 构造 JSON 请求体。
func jsonReader(v any) *bytes.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

// newDepsWithWriter 构造带真实组提交 writer 的依赖（验证落盘路径）。
func newDepsWithWriter(t *testing.T) (Deps, *writer.Writer) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	w := writer.New(dbs)
	w.Start()
	t.Cleanup(func() {
		_ = w.Stop()
		_ = dbs.Close()
	})

	st := store.New()
	enq := func(op ledger.Op) { w.Enqueue(op) }
	svc := &auth.Service{DBs: dbs, Store: st, Enqueue: enq, LockWindow: time.Minute}
	return Deps{
		Cfg:     config.Config{},
		DBs:     dbs,
		Store:   st,
		Auth:    svc,
		Enqueue: enq,
	}, w
}

type childInfo struct {
	ID int64 `json:"id"`
}

func mkChild(t *testing.T, h http.Handler, parentCookie *http.Cookie, uname, disp string) childInfo {
	t.Helper()
	rr := doJSON(h, "POST", "/api/children", map[string]any{
		"username": uname, "password": "password8", "displayName": disp,
	}, parentCookie)
	if rr.Code != 200 {
		t.Fatalf("建孩子 = %d %s", rr.Code, rr.Body.String())
	}
	var c childInfo
	json.Unmarshal(rr.Body.Bytes(), &c)
	return c
}

type itemInfo struct {
	ID int64 `json:"id"`
}

func mkItem(t *testing.T, h http.Handler, parentCookie *http.Cookie, title string, points int64) itemInfo {
	t.Helper()
	rr := doJSON(h, "POST", "/api/reward-items", map[string]any{
		"title": title, "points": points,
	}, parentCookie)
	if rr.Code != 200 {
		t.Fatalf("建积分项 = %d %s", rr.Code, rr.Body.String())
	}
	var it itemInfo
	json.Unmarshal(rr.Body.Bytes(), &it)
	return it
}

func TestEarnImmediateAndPersisted(t *testing.T) {
	deps, w := newDepsWithWriter(t)
	h := New(deps)
	p := regParent(t, deps.Auth, "earner1")
	pc := parentCookie(p.Token)

	it := mkItem(t, h, pc, "托管课写完作业", 7)
	child := mkChild(t, h, pc, "earnerkid", "娃")

	rr := doJSON(h, "POST", "/api/earnings", map[string]any{
		"childId": child.ID, "rewardItemId": it.ID, "note": "今天很棒",
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("earn = %d %s", rr.Code, rr.Body.String())
	}
	var entry struct {
		Entry struct {
			Delta        int64  `json:"delta"`
			BalanceAfter int64  `json:"balanceAfter"`
			Kind         string `json:"kind"`
			ItemTitle    string `json:"itemTitle"`
		} `json:"entry"`
	}
	json.Unmarshal(rr.Body.Bytes(), &entry)
	if entry.Entry.Delta != 7 || entry.Entry.BalanceAfter != 7 ||
		entry.Entry.Kind != "earn" || entry.Entry.ItemTitle != "托管课写完作业" {
		t.Errorf("earn 响应不符: %+v", entry.Entry)
	}

	// 内存即时生效
	if got := deps.Store.Child(child.ID).Bal(); got != 7 {
		t.Errorf("内存余额 = %d, want 7", got)
	}
	// 今日计数
	deps.Store.BumpEarnsToday(p.FamilyID, "1970-01-01") // 占位避免零值歧义
	// （真实 dayKey 断言在 overview 任务）

	// 落盘：排空后核对
	if err := w.Stop(); err != nil {
		t.Fatalf("writer.Stop: %v", err)
	}
	var delta, bal, refItem int64
	var kind string
	err := deps.DBs.Read.QueryRow(
		"SELECT delta, balance_after, kind, ref_reward_item_id FROM ledger WHERE child_id=?",
		child.ID,
	).Scan(&delta, &bal, &kind, &refItem)
	if err != nil {
		t.Fatalf("查 ledger: %v", err)
	}
	if delta != 7 || bal != 7 || kind != "earn" || refItem != it.ID {
		t.Errorf("落盘行不符: delta=%d bal=%d kind=%s ref=%d", delta, bal, kind, refItem)
	}
	var userBal int64
	deps.DBs.Read.QueryRow("SELECT balance FROM users WHERE id=?", child.ID).Scan(&userBal)
	if userBal != 7 {
		t.Errorf("users.balance = %d, want 7", userBal)
	}
}

func TestAdjustRejectNegative(t *testing.T) {
	deps, w := newDepsWithWriter(t)
	h := New(deps)
	p := regParent(t, deps.Auth, "earner2")
	pc := parentCookie(p.Token)
	it := mkItem(t, h, pc, "大分项", 50)
	child := mkChild(t, h, pc, "earnerkid2", "娃")

	doJSON(h, "POST", "/api/earnings", map[string]any{
		"childId": child.ID, "rewardItemId": it.ID,
	}, pc)

	// 调到负 → 409 零副作用（内存即时可验）
	rr := doJSON(h, "POST", "/api/adjustments", map[string]any{
		"childId": child.ID, "delta": -100, "note": "罚",
	}, pc)
	if rr.Code != 409 || errCode(t, rr) != "insufficient_points" {
		t.Fatalf("负余额调整 = %d %s, want 409 insufficient_points", rr.Code, rr.Body.String())
	}
	if got := deps.Store.Child(child.ID).Bal(); got != 50 {
		t.Errorf("拒绝后内存余额 = %d, want 50", got)
	}

	// 合法部分扣减
	rr = doJSON(h, "POST", "/api/adjustments", map[string]any{
		"childId": child.ID, "delta": -20, "note": "罚 20",
	}, pc)
	if rr.Code != 200 {
		t.Fatalf("合法调整 = %d %s", rr.Code, rr.Body.String())
	}
	var entry struct {
		Entry struct {
			Delta        int64  `json:"delta"`
			BalanceAfter int64  `json:"balanceAfter"`
			Kind         string `json:"kind"`
		} `json:"entry"`
	}
	json.Unmarshal(rr.Body.Bytes(), &entry)
	if entry.Entry.Delta != -20 || entry.Entry.BalanceAfter != 30 || entry.Entry.Kind != "adjust" {
		t.Errorf("adjust 响应不符: %+v", entry.Entry)
	}

	// note 缺失 → 422
	rr = doJSON(h, "POST", "/api/adjustments", map[string]any{
		"childId": child.ID, "delta": 5,
	}, pc)
	if rr.Code != 422 {
		t.Errorf("缺 note = %d, want 422", rr.Code)
	}

	// 落盘终态：earn +50 与 adjust -20 两行，拒绝的那笔不落盘
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	var cnt, sum int64
	deps.DBs.Read.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(delta),0) FROM ledger WHERE child_id=?", child.ID,
	).Scan(&cnt, &sum)
	if cnt != 2 || sum != 30 {
		t.Errorf("落盘 COUNT=%d SUM=%d, want 2/30", cnt, sum)
	}
	var userBal int64
	deps.DBs.Read.QueryRow("SELECT balance FROM users WHERE id=?", child.ID).Scan(&userBal)
	if userBal != 30 {
		t.Errorf("users.balance = %d, want 30", userBal)
	}
}

func TestEarnInactiveItem(t *testing.T) {
	deps, _ := newDepsWithWriter(t)
	h := New(deps)
	p := regParent(t, deps.Auth, "earner3")
	pc := parentCookie(p.Token)
	it := mkItem(t, h, pc, "被停项", 7)
	child := mkChild(t, h, pc, "earnerkid3", "娃")

	doJSON(h, "PATCH", apiPath("reward-items", it.ID), map[string]any{"active": false}, pc)

	rr := doJSON(h, "POST", "/api/earnings", map[string]any{
		"childId": child.ID, "rewardItemId": it.ID,
	}, pc)
	if rr.Code != 422 {
		t.Errorf("停用项 earn = %d %s, want 422", rr.Code, rr.Body.String())
	}

	// 不存在的孩子 → 404
	rr = doJSON(h, "POST", "/api/earnings", map[string]any{
		"childId": 999999, "rewardItemId": it.ID,
	}, pc)
	if rr.Code != 404 {
		t.Errorf("陌生孩子 earn = %d, want 404", rr.Code)
	}
}

func TestConcurrentEarn(t *testing.T) {
	deps, w := newDepsWithWriter(t)
	h := New(deps)
	p := regParent(t, deps.Auth, "earner4")
	pc := parentCookie(p.Token)
	it := mkItem(t, h, pc, "一分项", 1)
	child := mkChild(t, h, pc, "earnerkid4", "娃")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", "/api/earnings", jsonReader(map[string]any{
				"childId": child.ID, "rewardItemId": it.ID,
			}))
			req.AddCookie(pc)
			h.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}
	wg.Wait()

	if got := deps.Store.Child(child.ID).Bal(); got != 50 {
		t.Errorf("并发后内存余额 = %d, want 50", got)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	var sum, cnt int64
	deps.DBs.Read.QueryRow(
		"SELECT COALESCE(SUM(delta),0), COUNT(*) FROM ledger WHERE child_id=?", child.ID,
	).Scan(&sum, &cnt)
	if sum != 50 || cnt != 50 {
		t.Errorf("落盘 SUM=%d COUNT=%d, want 50/50", sum, cnt)
	}
	var userBal int64
	deps.DBs.Read.QueryRow("SELECT balance FROM users WHERE id=?", child.ID).Scan(&userBal)
	if userBal != 50 {
		t.Errorf("users.balance = %d, want 50", userBal)
	}
}
