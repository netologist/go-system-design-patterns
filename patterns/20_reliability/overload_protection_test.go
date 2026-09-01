package reliability_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	reliability "system-design-patterns/patterns/20_reliability"
)

func TestOverloadProtectionMiddleware_CapacityShedding(t *testing.T) {
	middleware := reliability.OverloadProtectionMiddleware(2) // Max 2 active requests

	blockCh := make(chan struct{})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blockCh
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup

	// Start 2 concurrent requests that block
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/work", nil)
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for allowed requests, got: %d", w.Code)
			}
		}()
	}

	// Wait briefly for the 2 requests to acquire slots
	time.Sleep(10 * time.Millisecond)

	// 3rd concurrent request while 2 are still in-flight -> MUST return 503
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/work", nil)
	handler.ServeHTTP(w3, req3)

	if w3.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 on saturated server, got: %d", w3.Code)
	}
	if w3.Header().Get("Retry-After") != "2" {
		t.Errorf("expected Retry-After 2 header, got: %s", w3.Header().Get("Retry-After"))
	}

	// Unblock active requests
	close(blockCh)
	wg.Wait()
}
