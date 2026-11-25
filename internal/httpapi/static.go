package httpapi

import (
	"io/fs"
	"net/http"
	"strings"
)

// RegisterStatic 把前端静态资源挂到 mux：
//   - GET /assets/* → 文件服务 + 一年 immutable 缓存（文件名带内容指纹）
//   - GET /（其余一切非 /api、非已注册路径）→ 存在文件则回文件，否则回 index.html
//     （SPA 路由兜底，Cache-Control: no-cache）
//   - /api/* 未匹配路径 → 404 JSON（绝不能被 SPA 兜底吃掉）
func RegisterStatic(mux *http.ServeMux, dist fs.FS) {
	fileServer := http.FileServerFS(dist)

	mux.Handle("GET /assets/", withCache(fileServer, "public, max-age=31536000, immutable"))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if st, err := fs.Stat(dist, p); err == nil && !st.IsDir() {
				w.Header().Set("Cache-Control", "no-cache")
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

func withCache(h http.Handler, cacheControl string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl)
		h.ServeHTTP(w, r)
	})
}
