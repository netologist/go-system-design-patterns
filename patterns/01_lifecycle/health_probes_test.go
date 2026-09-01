package lifecycle_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	lifecycle "system-design-patterns/patterns/01_lifecycle"
)

func TestProbeHandler_Startup(t *testing.T) {
	probes := lifecycle.NewProbeHandler(100 * time.Millisecond)

	// Initially not started
	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	w := httptest.NewRecorder()
	probes.StartupHandler()(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 before started, got: %d", w.Code)
	}

	// Mark as started
	probes.SetStarted(true)
	w = httptest.NewRecorder()
	probes.StartupHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 after started, got: %d", w.Code)
	}
}

func TestProbeHandler_Liveness(t *testing.T) {
	probes := lifecycle.NewProbeHandler(100 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	w := httptest.NewRecorder()
	probes.LivenessHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for default livez, got: %d", w.Code)
	}

	probes.SetLive(false)
	w = httptest.NewRecorder()
	probes.LivenessHandler()(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when live is false, got: %d", w.Code)
	}
}

func TestProbeHandler_ReadinessWithChecks(t *testing.T) {
	probes := lifecycle.NewProbeHandler(100 * time.Millisecond)
	probes.SetReady(true)

	dbHealthy := true
	probes.RegisterReadinessCheck("database", func(ctx context.Context) error {
		if !dbHealthy {
			return errors.New("db connection lost")
		}
		return nil
	})

	// When DB is healthy
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	probes.ReadinessHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 when checks pass, got: %d", w.Code)
	}

	// When DB fails
	dbHealthy = false
	w = httptest.NewRecorder()
	probes.ReadinessHandler()(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when readiness check fails, got: %d", w.Code)
	}
}
