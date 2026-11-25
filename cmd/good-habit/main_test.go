package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"good-habit/internal/app"
	"good-habit/internal/config"
	"good-habit/internal/db"
	"good-habit/migrations"
)

// fakeApp 是因果性测试替身：Shutdown 阻塞到测试放行，
// 用于确定性验证 run 不会在 Drain 完成前返回。
type fakeApp struct {
	listenRelease  chan struct{}
	shutdownCalled chan struct{}
	shutdownRel    chan struct{}
}

func (f *fakeApp) ListenAndServe() error {
	<-f.listenRelease
	return http.ErrServerClosed
}

func (f *fakeApp) Shutdown(ctx context.Context) error {
	select {
	case <-f.shutdownCalled:
	default:
		close(f.shutdownCalled)
	}
	<-f.shutdownRel
	return nil
}

// TestRunWaitsForShutdown 是 C1 的确定性复现：
// 旧实现（ListenAndServe 一返回就退出）会在 Shutdown 完成前返回。
func TestRunWaitsForShutdown(t *testing.T) {
	fa := &fakeApp{
		listenRelease:  make(chan struct{}),
		shutdownCalled: make(chan struct{}),
		shutdownRel:    make(chan struct{}),
	}
	sig := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() {
		done <- run(config.Config{}, sig, func(config.Config) (serverApp, error) {
			return fa, nil
		})
	}()

	// 触发停机路径：信号 → Shutdown（阻塞中）
	close(sig)
	select {
	case <-fa.shutdownCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown 未被调用")
	}
	// 放行 ListenAndServe：旧实现此刻会直接返回
	close(fa.listenRelease)

	select {
	case err := <-done:
		t.Fatalf("run 在 Shutdown/Drain 完成前就返回了（err=%v）——SIGTERM 丢写竞态", err)
	case <-time.After(150 * time.Millisecond):
		// 正确行为：run 仍在等待 Drain
	}

	// 放行 Shutdown → run 应随后返回
	close(fa.shutdownRel)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run 返回错误: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("放行后 run 未返回")
	}
}

// TestRunDrainsBeforeReturn 验证 run() 返回时优雅停机（含写队列 Drain）必须已完成：
// 注册会话 + 200 笔得分在 run 返回前必须全部落盘。旧行为（ListenAndServe 一返回
// 就退出进程）会在 Drain 完成前杀掉写协程——最后一批未提交的事件随之丢失。
func TestRunDrainsBeforeReturn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "run.db")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("找空闲端口: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cfg := config.Config{Addr: addr, DBPath: dbPath, ReadPool: 5}
	sig := make(chan os.Signal, 1)

	var exited int32
	runErr := make(chan error, 1)
	go func() {
		runErr <- run(cfg, sig, func(c config.Config) (serverApp, error) {
			return app.New(c)
		})
		atomic.StoreInt32(&exited, 1)
	}()

	post := func(path string, body any) int {
		b, _ := json.Marshal(body)
		resp, err := http.Post(fmt.Sprintf("http://%s%s", addr, path), "application/json", bytes.NewReader(b))
		if err != nil {
			return 0
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	// 等服务就绪并注册
	deadline := time.Now().Add(15 * time.Second)
	code := 0
	for time.Now().Before(deadline) {
		if code = post("/api/auth/register", map[string]string{
			"username": "runtest1", "password": "password8",
		}); code == 200 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if code != 200 {
		t.Fatalf("服务未就绪（最后状态 %d）", code)
	}

	// 建 cookie 会话（同源 cookie 自动携带）
	// 拿 cookie：登录一次
	b, _ := json.Marshal(map[string]string{"username": "runtest1", "password": "password8"})
	loginResp, err := http.Post(fmt.Sprintf("http://%s/api/auth/login", addr), "application/json", bytes.NewReader(b))
	if err != nil || loginResp.StatusCode != 200 {
		t.Fatalf("登录失败: %v %v", err, loginResp)
	}
	var cookies []*http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "gh_session" {
			cookies = append(cookies, c)
		}
	}
	loginResp.Body.Close()
	if len(cookies) == 0 {
		t.Fatal("登录应下发 cookie")
	}

	postAuth := func(path string, body any) int {
		bb, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s%s", addr, path), bytes.NewReader(bb))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code = postAuth("/api/children", map[string]string{
		"username": "runtestkid", "password": "password8", "displayName": "娃",
	}); code != 200 {
		t.Fatalf("建孩子 = %d", code)
	}
	var childID int64
	{
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/api/children", addr), nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("查孩子: %v", err)
		}
		var list struct {
			Children []struct {
				ID int64 `json:"id"`
			} `json:"children"`
		}
		json.NewDecoder(resp.Body).Decode(&list)
		resp.Body.Close()
		if len(list.Children) != 1 {
			t.Fatalf("孩子数 = %d", len(list.Children))
		}
		childID = list.Children[0].ID
	}

	// 200 笔得分（大量未提交事件制造 Drain 负载）
	for i := 0; i < 200; i++ {
		// 直接铺 ledger：用 earn 需要积分项；这里用 adjustments（note 必填）
		if code = postAuth("/api/adjustments", map[string]any{
			"childId": childID, "delta": 1, "note": "burst",
		}); code != 200 {
			t.Fatalf("第 %d 笔调整 = %d", i+1, code)
		}
	}

	// 触发停机信号
	close(sig)

	// 紧凑轮询等 run 返回（1ms 粒度，避免给旧竞态掩蔽窗口）
	exitDeadline := time.Now().Add(30 * time.Second)
	for atomic.LoadInt32(&exited) == 0 && time.Now().Before(exitDeadline) {
		time.Sleep(time.Millisecond)
	}
	if atomic.LoadInt32(&exited) == 0 {
		t.Fatal("run 未在 30 秒内返回")
	}
	if err := <-runErr; err != nil {
		t.Fatalf("run 返回错误: %v", err)
	}

	// 关键断言：Drain 必须已完成——200 笔 + 1 会话全部落盘
	probe, err := db.Open(dbPath, 2)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	defer probe.Close()
	if err := probe.Migrate(migrations.FS); err != nil {
		t.Fatalf("probe Migrate: %v", err)
	}
	var nLedger, nSessions int64
	probe.Read.QueryRow("SELECT COUNT(*) FROM ledger WHERE child_id=?", childID).Scan(&nLedger)
	probe.Read.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&nSessions)
	if nLedger != 200 {
		t.Fatalf("run 返回时账单行 = %d, want 200（Drain 未完成即退出）", nLedger)
	}
	if nSessions != 2 {
		t.Fatalf("run 返回时会话行 = %d, want 2（register+login 各一个）", nSessions)
	}
}
