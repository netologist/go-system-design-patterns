package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "system-design-patterns/patterns/11_middleware"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	mw := middleware.SecurityHeadersMiddleware
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/secure", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing or wrong X-Frame-Options")
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing or wrong X-Content-Type-Options")
	}
	if w.Header().Get("Content-Security-Policy") != "default-src 'self'" {
		t.Error("missing or wrong CSP")
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	cfg := middleware.CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}

	mw := middleware.CORSMiddleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/data", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS preflight, got: %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Errorf("expected allowed origin in header, got: %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}
