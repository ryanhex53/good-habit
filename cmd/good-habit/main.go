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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	if err := run(cfg, sig, func(c config.Config) (serverApp, error) {
		return app.New(c)
	}); err != nil {
		log.Fatal(err)
	}
}

// serverApp 是 run 需要的最小应用接口（测试注入 fake 用）。
type serverApp interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// run 启动服务并阻塞到优雅停机完成（含写队列 Drain）后返回。
// sig/newApp 由调用方注入（main 里是真实信号与 app.New；测试注入可控替身）。
func run(cfg config.Config, sig <-chan os.Signal, newApp func(config.Config) (serverApp, error)) error {
	a, err := newApp(cfg)
	if err != nil {
		return fmt.Errorf("启动: %w", err)
	}

	done := make(chan struct{})
	go func() {
		<-sig
		slog.Info("收到停机信号，开始 Drain")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.Shutdown(ctx); err != nil {
			slog.Error("优雅停机异常", "err", err)
		}
		close(done)
	}()

	if err := a.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("监听: %w", err)
	}
	<-done // 等 Drain 完成后才退出进程（否则 SIGTERM 丢已确认的写入）
	slog.Info("已退出")
	return nil
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
	fmt.Printf("用户 %s 的密码已重置（数据库中的会话已删除）\n", *user)
	fmt.Println("注意：正在运行的服务持有内存会话，请执行 systemctl restart good-habit 使旧会话彻底失效")
}
