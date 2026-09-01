# Kubernetes PreStop Draining & Zero-Downtime Termination Pattern

## 1. Overview & Concept
The **Kubernetes PreStop Draining Pattern** coordinates the graceful shutdown lifecycle of containerized Go backend services to achieve true **zero-downtime deployments** and rolling updates in Kubernetes clusters. When a Kubernetes Pod is terminated (e.g., during a rolling deploy, node drain, or autoscaler scale-down), Kubernetes executes several operations asynchronously in parallel:
1. It sends a `SIGTERM` signal to the container process.
2. It removes the Pod IP from the Service `Endpoints` / `EndpointSlice` object.
3. Kube-proxy, Ingress Controllers (NGINX, Traefik, Envoy), and cloud load balancers (AWS ALB, GCP Cloud Armor) receive the endpoint removal notification and update their internal routing tables.

Because network routing table updates take several seconds to propagate across all cluster nodes and cloud load balancers, a Pod that terminates immediately upon receiving `SIGTERM` will drop in-flight requests and cause HTTP 502/504 Bad Gateway errors for new requests routed to the terminating Pod during the propagation window.

This pattern leverages a **Container Lifecycle `preStop` hook** and readiness probe manipulation:
- Immediately mark the readiness probe as **Unready (`HTTP 503`)**.
- Sleep for a configurable draining delay (`drainingDelay`, e.g., 5-15 seconds) to allow all ingress controllers and load balancers to cease forwarding new traffic.
- Track active in-flight requests using atomic counters and drain remaining connections gracefully before process exit.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
Without a synchronized preStop draining phase, high-traffic backend services experience dropped connections and error spikes during every standard deployment.

### Failure Scenarios Without This Pattern
- **HTTP 502 Bad Gateway Spikes During Rolling Updates:** Ingress controllers continue forwarding new incoming requests to a Pod that has already closed its listening TCP socket, resulting in connection refused errors (`ECONNREFUSED`).
- **Abrupt Termination of Long-Running Requests:** Large file uploads, complex database transactions, or SSE streams are abruptly severed when the container exits prematurely.
- **Race Condition in Endpoint De-Registration:** Kubernetes does not wait for `Endpoints` propagation before sending `SIGTERM`. If the Go process exits in 50ms, requests in transit are dropped.
- **Cascading Client Retries:** Dropped requests trigger immediate client retries against remaining pods, causing CPU spikes during rollouts.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The timeline below illustrates how the `PreStopDrainingManager` coordinates with Kubernetes ingress propagation:

```
[ K8s Deployment Rolling Update Triggered ]
                     │
                     ▼
       ┌───────────────────────────────┐
       │   K8s Invokes Container       │
       │   preStop Hook Handler        │
       └──────────────┬────────────────┘
                     │
     1. isReady.Store(false)
        -> /readyz probe returns HTTP 503
                     │
     2. Wait drainingDelay (e.g. 5-15s)
        -> Kube-proxy & Ingress remove Pod from Endpoints
        -> Zero new requests routed to this Pod
                     │
     3. Active Requests Drain
        -> Existing in-flight requests finish cleanly
                     │
                     ▼
       [ preStop Hook Exits Cleanly ]
                     │
                     ▼
       [ K8s Sends SIGTERM to Go Process ]
                     │
                     ▼
       [ http.Server.Shutdown(ctx) ]
                     │
                     ▼
          [ Container Exits with 0 ]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Tune `drainingDelay` to Ingress Propagation Latency:** In cloud environments (e.g., AWS ALB Ingress Controller), target de-registration takes between 5 to 15 seconds. Ensure `drainingDelay` exceeds the maximum propagation delay.
- **Ensure `terminationGracePeriodSeconds` is Sufficient:** In your Kubernetes Pod spec, set `terminationGracePeriodSeconds` (default 30s) larger than `drainingDelay + serverShutdownTimeout` (e.g., 45-60s) to prevent Kubernetes from issuing a hard `SIGKILL`.
- **Expose Separate Liveness and Readiness Endpoints:** Use `/healthz` for liveness (remains 200 OK during shutdown to prevent premature restarts) and `/readyz` for readiness (flips to 503 during preStop draining).
- **Non-Zero In-Flight Request Tracking:** Use `atomic.Int64` middleware to monitor remaining in-flight requests and log progress during the drain phase.

### Pitfalls to Avoid
- **Failing Liveness Probe During PreStop:** If the liveness probe returns 503 during preStop, Kubernetes may restart the container mid-drain, causing data corruption.
- **Omitting `preStop` Configuration in Kubernetes YAML:** The Go application must expose the preStop HTTP endpoint or CLI command and configure it in the Pod manifest:
  ```yaml
  lifecycle:
    preStop:
      httpGet:
        path: /prestop
        port: 8080
  ```

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/21_deployment/prestop_draining.go`
```go
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
```

### Production Kubernetes HTTP Server Example
```go
func main() {
    drainingManager := deployment.NewPreStopDrainingManager(10 * time.Second)

    mux := http.NewServeMux()
    // Readiness probe for kubelet
    mux.HandleFunc("/readyz", drainingManager.ReadinessHandler())
    
    // PreStop hook endpoint invoked by K8s container lifecycle
    mux.HandleFunc("/prestop", func(w http.ResponseWriter, r *http.Request) {
        _ = drainingManager.PreStopHook(r.Context())
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("DRAIN_COMPLETE"))
    })

    // Business API routes wrapped with in-flight tracker
    apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Business logic...
    })
    mux.Handle("/api/", drainingManager.TrackRequest(apiHandler))

    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
    }

    go func() {
        if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Fatalf("server error: %v", err)
        }
    }()

    // Listen for OS SIGTERM from Kubernetes
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    <-quit

    // In-flight requests are already minimized by preStop; perform standard shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    _ = server.Shutdown(shutdownCtx)
}
```
