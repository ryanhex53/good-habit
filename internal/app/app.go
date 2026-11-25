// Package app 把全部组件装配成一个可优雅停机的进程（设计文档 §6.7/§6.6）：
// Open → Migrate → sweep → LoadAll → writer.Start → httpapi → 每小时 sweep。
// Shutdown 顺序：http.Server.Shutdown（停止接新请求）→ writer.Stop（Drain）
// → 停 sweep 协程 → dbs.Close。
package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"good-habit/internal/auth"
	"good-habit/internal/config"
	"good-habit/internal/db"
	"good-habit/internal/httpapi"
	"good-habit/internal/store"
	"good-habit/internal/sweep"
	"good-habit/internal/writer"
	"good-habit/migrations"
)

// App 是装配完成的进程句柄。
type App struct {
	cfg     config.Config
	dbs     *db.DBs
	store   *store.Store
	writer  *writer.Writer
	srv     *http.Server
	sweepStop chan struct{}
	sweepDone chan struct{}
}

// New 装配整个应用；返回后 HTTP 已可服务（调 ListenAndServe）。
func New(cfg config.Config) (*App, error) {
	dbs, err := db.Open(cfg.DBPath, cfg.ReadPool)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	if err := dbs.Migrate(migrations.FS); err != nil {
		_ = dbs.Close()
		return nil, fmt.Errorf("迁移: %w", err)
	}

	st := store.New()
	// 先清扫超时待选局，再重建内存态（顺带修复可能的余额偏差）
	if n, err := sweep.Run(dbs, st); err != nil {
		slog.Warn("启动清扫失败（继续启动）", "err", err)
	} else if n > 0 {
		slog.Info("启动清扫超时待选局", "n", n)
	}
	if err := st.LoadAll(dbs); err != nil {
		_ = dbs.Close()
		return nil, fmt.Errorf("重建内存态: %w", err)
	}

	w := writer.New(dbs)
	w.Start()

	svc := &auth.Service{DBs: dbs, Store: st, Enqueue: w.Enqueue}
	handler := httpapi.New(httpapi.Deps{
		Cfg:     cfg,
		DBs:     dbs,
		Store:   st,
		Auth:    svc,
		Enqueue: w.Enqueue,
		Flush:   w.Flush,
	})

	a := &App{
		cfg: cfg, dbs: dbs, store: st, writer: w,
		srv:       &http.Server{Addr: cfg.Addr, Handler: handler},
		sweepStop: make(chan struct{}),
		sweepDone: make(chan struct{}),
	}
	go a.sweepLoop()
	return a, nil
}

// Handler 暴露 HTTP 处理器（测试用）。
func (a *App) Handler() http.Handler { return a.srv.Handler }

// ListenAndServe 开始监听（阻塞）。
func (a *App) ListenAndServe() error { return a.srv.ListenAndServe() }

// Shutdown 优雅停机：Drain 全部在途写入后关库。幂等安全（二次调用只关库）。
func (a *App) Shutdown(ctx context.Context) error {
	var firstErr error
	if err := a.srv.Shutdown(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("http 停机: %w", err)
	}
	if err := a.writer.Stop(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("写队列 Drain: %w", err)
	}
	close(a.sweepStop)
	<-a.sweepDone
	if err := a.dbs.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (a *App) sweepLoop() {
	defer close(a.sweepDone)
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if n, err := sweep.Run(a.dbs, a.store); err != nil {
				slog.Warn("小时清扫失败", "err", err)
			} else if n > 0 {
				slog.Info("清扫超时待选局", "n", n)
			}
		case <-a.sweepStop:
			return
		}
	}
}

// ResetPassword 重置用户密码并踢掉其全部会话（CLI 子命令与测试共用）。
func ResetPassword(dbs *db.DBs, username, newPw string) error {
	if len(newPw) < 8 {
		return fmt.Errorf("新密码至少 8 位")
	}
	uname := strings.ToLower(strings.TrimSpace(username))
	hash, err := auth.HashPassword(newPw)
	if err != nil {
		return err
	}
	return dbs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE users SET password_hash=? WHERE username=?", hash, uname)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("用户 %q 不存在", username)
		}
		_, err = tx.Exec(
			"DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE username=?)", uname)
		return err
	})
}
