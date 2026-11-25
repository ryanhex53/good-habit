// Package httpapi 是 HTTP 层（设计文档 §6.9）：标准库方法路由、
// JSON 编解码、错误信封、中间件链（请求ID → 日志 → 限流 → 会话）、
// 角色门与静态资源（Task 23 接入）。
package httpapi

import (
	"net/http"
	"time"

	"good-habit/internal/auth"
	"good-habit/internal/config"
	"good-habit/internal/db"
	"good-habit/internal/ledger"
	"good-habit/internal/ratelimit"
	"good-habit/internal/store"
)

// Deps 是 httpapi 的全部依赖注入。
type Deps struct {
	Cfg     config.Config
	DBs     *db.DBs
	Store   *store.Store
	Auth    *auth.Service
	Enqueue func(ledger.Op)
	// Flush 是组提交屏障（写后读路径用；nil = 无 writer，跳过屏障）。
	Flush func() error

	// RateGeneral 是每 IP 每分钟全局请求上限；0 → 240。
	RateGeneral int64
}

// server 承载全部 handler；一个进程一个实例。
type server struct {
	deps           Deps
	generalLimiter *ratelimit.Limiter
	authLimiter    *ratelimit.Limiter // /api/auth/*：每 IP 10 次/分钟
	draw           *drawService
}

func newServer(deps Deps) *server {
	max := deps.RateGeneral
	if max <= 0 {
		max = 240
	}
	return &server{
		deps:           deps,
		generalLimiter: ratelimit.New(time.Minute, max),
		authLimiter:    ratelimit.New(time.Minute, 10),
		draw:           newDrawService(deps),
	}
}

// routes 注册全部端点并返回裸 mux（测试可直接追加路由）。
func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.handleMe)

	mux.HandleFunc("GET /api/children", RequireRole("parent", s.handleChildrenList))
	mux.HandleFunc("POST /api/children", RequireRole("parent", s.handleChildCreate))
	mux.HandleFunc("PATCH /api/children/{id}", RequireRole("parent", s.handleChildUpdate))

	mux.HandleFunc("GET /api/reward-items", RequireRole("parent", s.handleItemsList))
	mux.HandleFunc("POST /api/reward-items", RequireRole("parent", s.handleItemCreate))
	mux.HandleFunc("PATCH /api/reward-items/{id}", RequireRole("parent", s.handleItemUpdate))
	mux.HandleFunc("DELETE /api/reward-items/{id}", RequireRole("parent", s.handleItemDelete))

	mux.HandleFunc("GET /api/prizes", RequireRole("parent", s.handlePrizesList))
	mux.HandleFunc("POST /api/prizes", RequireRole("parent", s.handlePrizeCreate))
	mux.HandleFunc("PATCH /api/prizes/{id}", RequireRole("parent", s.handlePrizeUpdate))
	mux.HandleFunc("DELETE /api/prizes/{id}", RequireRole("parent", s.handlePrizeDelete))

	mux.HandleFunc("GET /api/wheel-config", RequireRole("parent", s.handleWheelConfigGet))
	mux.HandleFunc("PUT /api/wheel-config", RequireRole("parent", s.handleWheelConfigPut))

	mux.HandleFunc("POST /api/bootstrap", RequireRole("parent", s.handleBootstrap))

	mux.HandleFunc("POST /api/earnings", RequireRole("parent", s.handleEarning))
	mux.HandleFunc("POST /api/adjustments", RequireRole("parent", s.handleAdjustment))

	mux.HandleFunc("POST /api/draws", RequireRole("child", s.handleDraw))
	mux.HandleFunc("POST /api/draws/{uuid}/choose", RequireRole("child", s.handleChoose))
	mux.HandleFunc("PATCH /api/draws/{uuid}/fulfill", RequireRole("parent", s.handleFulfill))
	// 后续任务在此追加 /api/* 路由
	return mux
}

// New 返回完整中间件链包装的 http.Handler。
// 注：计划原文写返回 *http.ServeMux，但链必须包在 mux 外层，
// 故返回 http.Handler（ServeHTTP/http.Server.Handler 均可直接消费）。
func New(deps Deps) http.Handler {
	s := newServer(deps)
	return s.chain(s.routes())
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
