// good-habit 入口：最小启动（/healthz）。
// Task 17 会重写为全装配（DB/内存热路径/组提交/优雅停机）。
package main

import (
	"log"
	"net/http"
	"os"

	"good-habit/internal/config"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		log.Fatalf("加载配置: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("good-habit listening on %s (db: %s)", cfg.Addr, cfg.DBPath)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}
