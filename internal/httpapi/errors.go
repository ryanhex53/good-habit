package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// APIError 是 handler 可返回的结构化业务错误。
type APIError struct {
	Status int
	Code   string
	Msg    string
}

// Error 实现 error 接口。
func (e *APIError) Error() string { return e.Code + ": " + e.Msg }

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr 输出统一错误信封：{"error":{"code":...,"message":...}}。
func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

// writeInternal 记录底层错误（含请求 ID/路径）后返回 500 信封——
// 生产环境从 journalctl 可诊断，而不是只见 "internal"。
func writeInternal(w http.ResponseWriter, r *http.Request, err error, msg string) {
	slog.Error("internal", "err", err, "req", r.Header.Get("X-Request-Id"), "path", r.URL.Path)
	writeErr(w, http.StatusInternalServerError, "internal", msg)
}

// decodeJSON 读取请求体并限制大小；失败返回 false（响应已写）。
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "invalid_json", "请求体不是合法 JSON")
		return false
	}
	return true
}
