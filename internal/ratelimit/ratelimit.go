// Package ratelimit 实现内存定长窗口限流（设计文档 §6.8）：
// epoch = now/window 分桶，key（IP / 用户名 / 孩子ID）各自计数；
// 后台协程每 30s 清扫过期桶。全部计数走 atomic，读路径零磁盘。
package ratelimit

import (
	"sync"
	"sync/atomic"
	"time"
)

type slot struct {
	epoch int64
	count atomic.Int64
}

// Limiter 是一个（窗口, 上限）组合的限流器。
type Limiter struct {
	windowMs int64
	max      int64

	mu    sync.Mutex
	slots map[string]*slot

	stopOnce  sync.Once
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// New 创建限流器并启动 30s 清扫协程；不再使用时 Stop。
func New(window time.Duration, max int64) *Limiter {
	l := &Limiter{
		windowMs:  window.Milliseconds(),
		max:       max,
		slots:     map[string]*slot{},
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
	go l.janitor()
	return l
}

// Allow 记一次访问并判定是否放行。
func (l *Limiter) Allow(key string) bool {
	if l.windowMs <= 0 || l.max <= 0 {
		return true
	}
	epoch := time.Now().UnixMilli() / l.windowMs

	l.mu.Lock()
	s, ok := l.slots[key]
	if !ok || s.epoch != epoch {
		s = &slot{epoch: epoch}
		l.slots[key] = s
	}
	l.mu.Unlock()
	return s.count.Add(1) <= l.max
}

// Stop 停掉清扫协程（幂等）。
func (l *Limiter) Stop() {
	l.stopOnce.Do(func() { close(l.stopCh) })
	<-l.stoppedCh
}

func (l *Limiter) janitor() {
	defer close(l.stoppedCh)
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			cur := time.Now().UnixMilli() / l.windowMs
			l.mu.Lock()
			for k, s := range l.slots {
				if s.epoch < cur {
					delete(l.slots, k)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}
