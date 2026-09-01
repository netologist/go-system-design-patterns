package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "system-design-patterns/patterns/11_middleware"
)

func TestMetricsMiddleware_Recording(t *testing.T) {
	metrics := middleware.NewHTTPMetrics()
	mw := middleware.MetricsMiddleware(metrics)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// 2 successful requests
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ok", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ok", nil))

	// 1 error request
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/error", nil))

	if metrics.TotalRequests() != 3 {
		t.Errorf("expected 3 total requests, got: %d", metrics.TotalRequests())
	}
	if metrics.TotalErrors() != 1 {
		t.Errorf("expected 1 total error, got: %d", metrics.TotalErrors())
	}
}
