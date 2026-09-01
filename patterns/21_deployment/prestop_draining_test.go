package deployment_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	deployment "system-design-patterns/patterns/21_deployment"
)

func TestPreStopDrainingManager_Lifecycle(t *testing.T) {
	mgr := deployment.NewPreStopDrainingManager(30 * time.Millisecond)

	// 1. Initially Ready
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	mgr.ReadinessHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK initially, got: %d", w.Code)
	}

	// 2. Trigger preStop hook
	start := time.Now()
	err := mgr.PreStopHook(context.Background())
	if err != nil {
		t.Fatalf("preStop hook failed: %v", err)
	}

	if time.Since(start) < 25*time.Millisecond {
		t.Errorf("expected draining delay duration respected")
	}

	// 3. Readiness probe must now return 503
	w = httptest.NewRecorder()
	mgr.ReadinessHandler()(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable after preStop hook, got: %d", w.Code)
	}
}
