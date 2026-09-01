package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "system-design-patterns/patterns/11_middleware"
)

func TestRateLimitMiddleware_IPThrottling(t *testing.T) {
	limiter := middleware.NewIPRateLimiter(5, 2) // 2 requests burst
	mw := middleware.RateLimitMiddleware(limiter)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1st request -> 200 OK
	req1 := httptest.NewRequest("GET", "/api", nil)
	req1.Header.Set("X-Forwarded-For", "192.168.1.10")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("expected request 1 to succeed, got: %d", w1.Code)
	}

	// 2nd request -> 200 OK
	req2 := httptest.NewRequest("GET", "/api", nil)
	req2.Header.Set("X-Forwarded-For", "192.168.1.10")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected request 2 to succeed, got: %d", w2.Code)
	}

	// 3rd request -> 429 Too Many Requests
	req3 := httptest.NewRequest("GET", "/api", nil)
	req3.Header.Set("X-Forwarded-For", "192.168.1.10")
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("expected request 3 to return 429, got: %d", w3.Code)
	}
	if w3.Header().Get("Retry-After") != "1" {
		t.Errorf("expected Retry-After header, got: %s", w3.Header().Get("Retry-After"))
	}
}
