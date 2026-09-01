package lifecycle

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HealthStatus represents the status of a health check.
type HealthStatus string

const (
	StatusHealthy   HealthStatus = "UP"
	StatusUnhealthy HealthStatus = "DOWN"
)

// HealthCheckFunc is a function that performs a single component health evaluation.
type HealthCheckFunc func(ctx context.Context) error

// CheckDetail holds the result of a single probe component.
type CheckDetail struct {
	Status HealthStatus `json:"status"`
	Error  string       `json:"error,omitempty"`
}

// HealthResponse represents standard health check JSON payload.
type HealthResponse struct {
	Status  HealthStatus           `json:"status"`
	Details map[string]CheckDetail `json:"details,omitempty"`
}

// ProbeHandler coordinates liveness, readiness, and startup checks.
type ProbeHandler struct {
	mu           sync.RWMutex
	readyChecks  map[string]HealthCheckFunc
	liveChecks   map[string]HealthCheckFunc
	isLive       atomic.Bool
	isReady      atomic.Bool
	isStarted    atomic.Bool
	checkTimeout time.Duration
}

// NewProbeHandler returns an initialized ProbeHandler.
func NewProbeHandler(checkTimeout time.Duration) *ProbeHandler {
	h := &ProbeHandler{
		readyChecks:  make(map[string]HealthCheckFunc),
		liveChecks:   make(map[string]HealthCheckFunc),
		checkTimeout: checkTimeout,
	}
	h.isLive.Store(true)
	h.isReady.Store(false)
	h.isStarted.Store(false)
	return h
}

// SetStarted sets the startup probe state.
func (h *ProbeHandler) SetStarted(started bool) {
	h.isStarted.Store(started)
}

// SetReady sets the overall readiness flag.
func (h *ProbeHandler) SetReady(ready bool) {
	h.isReady.Store(ready)
}

// SetLive sets the overall liveness flag.
func (h *ProbeHandler) SetLive(live bool) {
	h.isLive.Store(live)
}

// RegisterReadinessCheck registers a named dynamic check evaluated on readiness probe.
func (h *ProbeHandler) RegisterReadinessCheck(name string, check HealthCheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readyChecks[name] = check
}

// RegisterLivenessCheck registers a named dynamic check evaluated on liveness probe.
func (h *ProbeHandler) RegisterLivenessCheck(name string, check HealthCheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.liveChecks[name] = check
}

// StartupHandler serves /startupz
func (h *ProbeHandler) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !h.isStarted.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(HealthResponse{Status: StatusUnhealthy})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: StatusHealthy})
	}
}

// LivenessHandler serves /livez
func (h *ProbeHandler) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !h.isLive.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(HealthResponse{Status: StatusUnhealthy})
			return
		}

		h.mu.RLock()
		checks := make(map[string]HealthCheckFunc, len(h.liveChecks))
		for k, v := range h.liveChecks {
			checks[k] = v
		}
		h.mu.RUnlock()

		resp, ok := h.executeChecks(r.Context(), checks)
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// ReadinessHandler serves /readyz
func (h *ProbeHandler) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !h.isReady.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(HealthResponse{Status: StatusUnhealthy})
			return
		}

		h.mu.RLock()
		checks := make(map[string]HealthCheckFunc, len(h.readyChecks))
		for k, v := range h.readyChecks {
			checks[k] = v
		}
		h.mu.RUnlock()

		resp, ok := h.executeChecks(r.Context(), checks)
		if !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (h *ProbeHandler) executeChecks(parentCtx context.Context, checks map[string]HealthCheckFunc) (HealthResponse, bool) {
	ctx, cancel := context.WithTimeout(parentCtx, h.checkTimeout)
	defer cancel()

	details := make(map[string]CheckDetail)
	allHealthy := true

	for name, check := range checks {
		if err := check(ctx); err != nil {
			allHealthy = false
			details[name] = CheckDetail{
				Status: StatusUnhealthy,
				Error:  err.Error(),
			}
		} else {
			details[name] = CheckDetail{
				Status: StatusHealthy,
			}
		}
	}

	status := StatusHealthy
	if !allHealthy {
		status = StatusUnhealthy
	}

	return HealthResponse{
		Status:  status,
		Details: details,
	}, allHealthy
}
