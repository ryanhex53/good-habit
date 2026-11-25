// good-habit 入口：
//   good-habit                 启动服务（SIGINT/SIGTERM 优雅停机）
//   good-habit reset-password -user <name> -new <pw>   重置密码（服务器本地执行）
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"good-habit/internal/app"
	"good-habit/internal/config"
	"good-habit/internal/db"
	"good-habit/migrations"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		runResetPassword(os.Args[2:])
		return
	}

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatalf("加载配置: %v", err)
	}
	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("启动: %v", err)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		slog.Info("收到停机信号，开始 Drain")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.Shutdown(ctx); err != nil {
			slog.Error("优雅停机异常", "err", err)
		}
	}()

	if err := a.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("监听: %v", err)
	}
	slog.Info("已退出")
}

func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	user := fs.String("user", "", "用户名")
	newPw := fs.String("new", "", "新密码（≥8 位）")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("解析参数: %v", err)
	}
	if *user == "" || *newPw == "" {
		log.Fatal("需要 -user <用户名> -new <新密码>")
	}

	cfg, err := config.Load(nil)
	if err != nil {
		log.Fatalf("加载配置: %v", err)
	}
	dbs, err := db.Open(cfg.DBPath, cfg.ReadPool)
	if err != nil {
		log.Fatalf("打开数据库: %v", err)
	}
	defer dbs.Close()
	if err := dbs.Migrate(migrations.FS); err != nil {
		log.Fatalf("迁移: %v", err)
	}
	if err := app.ResetPassword(dbs, *user, *newPw); err != nil {
		log.Fatalf("重置密码: %v", err)
	}
	fmt.Printf("用户 %s 的密码已重置（其全部会话已失效）\n", *user)
}
