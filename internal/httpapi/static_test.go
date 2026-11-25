package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

import "testing/fstest"

func staticHandler(t *testing.T) http.Handler {
	t.Helper()
	dist := fstest.MapFS{
		"index.html":   &fstest.MapFile{Data: []byte("<!doctype html><title>good-habit</title>")},
		"assets/a.js":  &fstest.MapFile{Data: []byte("console.log(1)")},
		"favicon.ico":  &fstest.MapFile{Data: []byte("ico")},
	}
	s := newServer(Deps{Static: dist})
	return s.chain(s.routes())
}

func TestStaticServesIndex(t *testing.T) {
	h := staticHandler(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "good-habit") {
		t.Errorf("GET / = %d %q, want 200 index", rr.Code, rr.Body.String())
	}
}

func TestSPAFallback(t *testing.T) {
	h := staticHandler(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/kid/wheel", nil))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "good-habit") {
		t.Errorf("GET /kid/wheel = %d %q, want 200 index（SPA 兜底）", rr.Code, rr.Body.String())
	}
}

func TestAssetsImmutable(t *testing.T) {
	h := staticHandler(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/assets/a.js", nil))
	if rr.Code != 200 || rr.Body.String() != "console.log(1)" {
		t.Errorf("GET /assets/a.js = %d %q", rr.Code, rr.Body.String())
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("assets Cache-Control = %q", cc)
	}
}

func TestAPINotHijacked(t *testing.T) {
	h := staticHandler(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/nope = %d, want 404", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"not_found"`) {
		t.Errorf("应为 JSON 404: %q", rr.Body.String())
	}
}

func TestNonAssetFileNoCache(t *testing.T) {
	h := staticHandler(t)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/favicon.ico", nil))
	if rr.Code != 200 || rr.Body.String() != "ico" {
		t.Fatalf("GET /favicon.ico = %d %q", rr.Code, rr.Body.String())
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("非 hash 文件应 no-cache, got %q", cc)
	}
}
