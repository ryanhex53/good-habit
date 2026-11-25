// Package config 汇总进程全部可调参数（设计文档 §6.1、§13）：
// env 为底、命令行 flag 覆盖。
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 是 good-habit 的运行配置。
type Config struct {
	Addr         string // 监听地址，默认 127.0.0.1:8080
	DBPath       string // SQLite 数据库文件路径，默认 good-habit.db
	DayTZ        string // "今日"日界的 IANA 时区名；空 = 服务器本地时区
	TrustProxy   bool   // 信任 X-Forwarded-For（挂在隧道/反代后时开启）
	SecureCookie bool   // 会话 Cookie 加 Secure（TLS 前置后开启）
	ReadPool     int    // ReadDB 连接数，默认 5，clamp 到 [4,6]
}

// Load 从环境变量读基线值，再用 args 里的 flag 覆盖。
func Load(args []string) (Config, error) {
	cfg := Config{
		Addr:         envString("GOODHABIT_ADDR", "127.0.0.1:8080"),
		DBPath:       envString("GOODHABIT_DBPATH", "good-habit.db"),
		DayTZ:        strings.TrimSpace(os.Getenv("GOODHABIT_DAY_TZ")),
		TrustProxy:   envBool("GOODHABIT_TRUST_PROXY"),
		SecureCookie: envBool("GOODHABIT_SECURE_COOKIE"),
		ReadPool:     envInt("GOODHABIT_READ_POOL", 5),
	}

	fs := flag.NewFlagSet("good-habit", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "监听地址")
	fs.StringVar(&cfg.DBPath, "db", cfg.DBPath, "SQLite 数据库路径")
	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("解析启动参数: %w", err)
	}

	// 读池连接数收敛到 [4,6]（设计文档 §6.2）。
	if cfg.ReadPool < 4 {
		cfg.ReadPool = 4
	}
	if cfg.ReadPool > 6 {
		cfg.ReadPool = 6
	}
	return cfg, nil
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true":
		return true
	default:
		return false
	}
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
