package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware "system-design-patterns/patterns/11_middleware"
)

func TestTimeoutMiddleware_FastHandler(t *testing.T) {
	mw := middleware.TimeoutMiddleware(100 * time.Millisecond)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fast response"))
	}))

	req := httptest.NewRequest("GET", "/fast", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got: %d", w.Code)
	}
}

func TestTimeoutMiddleware_SlowHandlerTimesOut(t *testing.T) {
	mw := middleware.TimeoutMiddleware(20 * time.Millisecond)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			// Context canceled as expected
			return
		}
	}))

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status 504 Gateway Timeout, got: %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "gateway_timeout") {
		t.Errorf("expected timeout JSON message, got: %s", body)
	}
}
