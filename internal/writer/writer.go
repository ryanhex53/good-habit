// Package writer 实现异步组提交落盘（设计文档 §6.5）：
// 容量 8192 的缓冲 channel 接收写入事件，单一后台协程
// 凑满 maxBatch（默认 64）条或每 flushInterval（默认 15ms）一个
// SQLite 事务批量提交；Stop() 排空队列后返回（优雅停机零丢数据）。
package writer

import (
	"database/sql"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"good-habit/internal/db"
	"good-habit/internal/ledger"
)

const (
	queueSize    = 8192
	defaultBatch = 64
	defaultFlush = 15 * time.Millisecond
)

// Writer 是后台组提交写协程的句柄。
type Writer struct {
	dbs          *db.DBs
	events       chan ledger.Op
	flushInterval time.Duration
	maxBatch     int

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopping atomic.Bool

	commits    atomic.Int64
	errMu      sync.Mutex
	flushErr   error
	startOnce  sync.Once
	stopOnce   sync.Once
}

// Option 调整 Writer 行为（主要供测试注入可控时序）。
type Option func(*Writer)

// WithFlushInterval 设置批提交间隔（默认 15ms）。
func WithFlushInterval(d time.Duration) Option {
	return func(w *Writer) { w.flushInterval = d }
}

// WithMaxBatch 设置单事务最大事件数（默认 64）。
func WithMaxBatch(n int) Option {
	return func(w *Writer) { w.maxBatch = n }
}

// New 创建 Writer；调用 Start 启动后台协程，退出前必须 Stop。
func New(dbs *db.DBs, opts ...Option) *Writer {
	w := &Writer{
		dbs:           dbs,
		events:        make(chan ledger.Op, queueSize),
		flushInterval: defaultFlush,
		maxBatch:      defaultBatch,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	for _, o := range opts {
		o(w)
	}
	return w
}

// Enqueue 把一个落盘算子排进队列。队列满（8192）时阻塞——
// 热路径退化为背压限速而不是报错。
// Stop 之后再调用 Enqueue 属编程错误（生产者必须先于 writer 停止），直接 panic。
func (w *Writer) Enqueue(op ledger.Op) {
	if w.stopping.Load() {
		panic("writer: Enqueue after Stop")
	}
	w.events <- op
}

// Start 启动后台写协程（幂等）。
func (w *Writer) Start() {
	w.startOnce.Do(func() { go w.loop() })
}

// Stop 通知停机并 Drain：排空队列内剩余事件、提交最后一批后返回。
// 幂等。返回最后一次批次提交的错误（若有）。
func (w *Writer) Stop() error {
	w.stopOnce.Do(func() {
		w.stopping.Store(true)
		w.Start() // 保证协程存在以执行 Drain（从未 Start 也安全）
		close(w.stopCh)
		<-w.doneCh
	})
	w.errMu.Lock()
	defer w.errMu.Unlock()
	return w.flushErr
}

// Commits 返回已提交的事务批数（测试钩子）。
func (w *Writer) Commits() int64 { return w.commits.Load() }

func (w *Writer) loop() {
	defer close(w.doneCh)
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]ledger.Op, 0, w.maxBatch)
	for {
		select {
		case op := <-w.events:
			batch = append(batch, op)
			if len(batch) >= w.maxBatch {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.flush(batch)
				batch = batch[:0]
			}
		case <-w.stopCh:
			// Drain：把通道里剩余的事件全部取出并分批提交。
			for {
				select {
				case op := <-w.events:
					batch = append(batch, op)
					if len(batch) >= w.maxBatch {
						w.flush(batch)
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						w.flush(batch)
						batch = batch[:0]
					}
					return
				}
			}
		}
	}
}

// flush 把一批事件合并成一个事务提交（WriteMu 与同步 CRUD 互斥）。
func (w *Writer) flush(batch []ledger.Op) {
	err := w.dbs.WithTx(func(tx *sql.Tx) error {
		for _, op := range batch {
			if err := op.Apply(tx); err != nil {
				return err
			}
		}
		return nil
	})
	w.commits.Add(1)
	if err != nil {
		w.errMu.Lock()
		w.flushErr = err
		w.errMu.Unlock()
		slog.Error("组提交批次失败", "err", err, "batch", len(batch))
	}
}
