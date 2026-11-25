package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAllowLimit(t *testing.T) {
	l := New(60*time.Second, 3)
	defer l.Stop()

	for i := 0; i < 3; i++ {
		if !l.Allow("ip-1") {
			t.Fatalf("第 %d 次应放行", i+1)
		}
	}
	if l.Allow("ip-1") {
		t.Error("第 4 次应拒绝")
	}
	// 异 key 互不影响
	if !l.Allow("ip-2") {
		t.Error("另一 key 应独立计数")
	}
}

func TestWindowRollover(t *testing.T) {
	l := New(50*time.Millisecond, 1)
	defer l.Stop()

	if !l.Allow("k") {
		t.Fatal("首次应放行")
	}
	if l.Allow("k") {
		t.Fatal("同窗口第二次应拒绝")
	}
	time.Sleep(120 * time.Millisecond)
	if !l.Allow("k") {
		t.Error("进入新窗口应重新放行")
	}
}

func TestConcurrentAllow(t *testing.T) {
	l := New(60*time.Second, 10)
	defer l.Stop()

	var wg sync.WaitGroup
	var allowed atomic.Int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Allow("same") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 10 {
		t.Errorf("100 并发中放行 = %d, want 恰 10", got)
	}
}

func TestStopIdempotent(t *testing.T) {
	l := New(time.Second, 5)
	l.Stop()
	l.Stop() // 不应 panic
}
