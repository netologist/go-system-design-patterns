package deployment

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// PreStopDrainingManager coordinates Kubernetes pod termination lifecycle:
// 1. preStop hook invoked by K8s
// 2. Mark readiness probe false (stops new traffic from ingress/kube-proxy)
// 3. Sleep drainingDelay (allows ingress controllers to remove Pod from endpoint list)
// 4. Drain remaining active in-flight requests gracefully
type PreStopDrainingManager struct {
	isReady        atomic.Bool
	activeRequests atomic.Int64
	drainingDelay  time.Duration
}

func NewPreStopDrainingManager(drainingDelay time.Duration) *PreStopDrainingManager {
	m := &PreStopDrainingManager{
		drainingDelay: drainingDelay,
	}
	m.isReady.Store(true)
	return m
}

// ReadinessHandler serves the Kubernetes readiness probe.
func (m *PreStopDrainingManager) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.isReady.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("UNREADY_DRAINING"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("READY"))
	}
}

// TrackRequest middleware tracking active requests.
func (m *PreStopDrainingManager) TrackRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.activeRequests.Add(1)
		defer m.activeRequests.Add(-1)
		next.ServeHTTP(w, r)
	})
}

// PreStopHook is invoked by Kubernetes container lifecycle preStop hook.
func (m *PreStopDrainingManager) PreStopHook(ctx context.Context) error {
	// 1. Mark unready so K8s router stops forwarding new requests
	m.isReady.Store(false)

	// 2. Sleep for kube-proxy / ingress propagation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(m.drainingDelay):
	}

	return nil
}

func (m *PreStopDrainingManager) ActiveRequests() int64 {
	return m.activeRequests.Load()
}
