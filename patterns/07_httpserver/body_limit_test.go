package httpserver_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpserver "system-design-patterns/patterns/07_httpserver"
)

func TestBodyLimitMiddleware_ExceedsLimit(t *testing.T) {
	limit := int64(20) // 20 bytes max
	middleware := httpserver.BodyLimitMiddleware(limit)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Payload larger than 20 bytes (e.g. 50 bytes)
	largeBody := strings.Repeat("A", 50)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(largeBody))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got: %d", w.Code)
	}
}

func TestRequireContentType(t *testing.T) {
	middleware := httpserver.RequireContentType("application/json")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Valid application/json with charset
	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid JSON content type, got: %d", w.Code)
	}

	// 2. Wrong Content-Type
	req = httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`hello`))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415 for text/plain, got: %d", w.Code)
	}

	// 3. Missing Content-Type
	req = httptest.NewRequest(http.MethodPost, "/api", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415 for missing content-type, got: %d", w.Code)
	}
}
