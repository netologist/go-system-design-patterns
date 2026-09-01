# Health Probes Pattern (Liveness, Readiness, Startup)

## 1. Overview & Concept
The **Health Probes** pattern provides standardized HTTP/RPC endpoints (`/livez`, `/readyz`, `/startupz`) that container orchestrators (such as Kubernetes) and load balancers use to inspect the internal operational lifecycle state of an application instance.

By decoupling probe responsibilities into three distinct health dimensions, the application provides precise control signals to the orchestrator:
1. **Startup Probe (`/startupz`)**: Informs the orchestrator whether the application has completed its internal bootstrap, dependency validation, and cache warming. Failure prevents liveness and readiness probes from executing and avoids premature container restarts for slow-starting services.
2. **Liveness Probe (`/livez`)**: Determines whether the Go runtime process is alive and responsive (e.g., event loop active, no internal deadlocks). Failure signals Kubernetes to terminate and restart the unhealthy container.
3. **Readiness Probe (`/readyz`)**: Determines whether the application is currently able to accept external client traffic (e.g., database connectivity alive, circuit breakers closed, graceful shutdown not in progress). Failure signals Kubernetes to remove the pod IP from load balancer endpoints without terminating the process.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Conflating or improperly implementing health probe semantics causes catastrophic cluster-wide outages:

- **Liveness Death Spirals & Cascading Outages**: Checking downstream databases or external APIs inside `/livez`. If PostgreSQL experiences a temporary spike in latency or connection exhaustion, all application pods fail liveness simultaneously. Kubernetes restarts every pod at once, flooding the recovering database with thousands of new reconnection handshakes and cementing a total outage.
- **Premature Traffic Ingestion**: Routing traffic to a freshly deployed pod before caches have warmed or connection pools have initialized, causing sudden bursts of `500 Internal Server Error` responses.
- **Unclean Drain During Rolling Updates**: When a pod receives `SIGTERM`, it must immediately fail its readiness probe so the Kubernetes Service endpoints remove the pod before active connections are closed. If readiness remains 200 OK during teardown, the load balancer continues routing new HTTP requests to the terminating pod.
- **Probe Denial-of-Service**: Executing heavy database queries (`SELECT count(*) FROM large_table`) on `/readyz` every 5 seconds across 100 pods generates thousands of unnecessary database queries, starving business workloads of database CPU.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Probe Decision Tree & Lifecycle Sequence

```
Container Boot ----------------> /startupz (Startup Probe)
                                      |
                                      +--> Returns 503 -> Keep waiting (no restart until failureThreshold)
                                      |
                                      +--> Returns 200 (isStarted = true)
                                              |
                     +------------------------+------------------------+
                     |                                                 |
                     v                                                 v
             /livez (Liveness Probe)                          /readyz (Readiness Probe)
                     |                                                 |
         Is runtime deadlocked / hung?                  Are DBs & dependencies reachable?
                     |                                  Is graceful shutdown in progress?
         +-----------+-----------+                             +-------+-------+
         |                       |                             |               |
     [Healthy]              [Deadlocked]                   [Ready]        [Unready]
         |                       |                             |               |
    HTTP 200 OK             HTTP 503 Error                HTTP 200 OK     HTTP 503 Error
  (Keep Running)          (Kubernetes Restarts Pod)     (Accept Traffic) (Remove from Endpoints)
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                            PROBE HANDLER                                |
 |                                                                         |
 |  Atomic State Toggles:                                                  |
 |  - isStarted: atomic.Bool (Controls /startupz)                          |
 |  - isLive:    atomic.Bool (Controls /livez base state)                  |
 |  - isReady:   atomic.Bool (Controls /readyz traffic gate)               |
 |                                                                         |
 |  Dynamic Health Checks:                                                 |
 |  - Liveness Checks:  map[string]HealthCheckFunc (Runtime deadlocks only)|
 |  - Readiness Checks: map[string]HealthCheckFunc (DB, Redis, Queues)     |
 |                                                                         |
 |  Handler Execution Pipeline:                                            |
 |  1. Evaluate Atomic Flag (If false -> Return 503 Service Unavailable)   |
 |  2. Derive Probe Timeout Context: context.WithTimeout(ctx, probeTimeout)|
 |  3. Execute Registered Checks Concurrently:                             |
 |     +--> On Check Success -> Record Status: "UP"                        |
 |     +--> On Check Failure -> Record Status: "DOWN", Error Detail        |
 |  4. If any check failed -> Return HTTP 503 + HealthResponse JSON        |
 |  5. If all checks pass  -> Return HTTP 200 + HealthResponse JSON        |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Keep Liveness Lightweight**: Liveness checks should only verify local process responsiveness (e.g., memory within bounds, goroutine leak checks, lock responsiveness). Never make network calls to external dependencies inside `/livez`.
- **Enforce Per-Probe Timeouts**: Always execute health checks inside a bounded context (e.g., 1–2 seconds) so slow database responses never cause probe requests to hang and exhaust HTTP server workers.
- **Fail Readiness Immediately on Shutdown**: In the signal handler or graceful shutdown trigger, flip `SetReady(false)` *before* draining in-flight requests. This guarantees that Kubernetes endpoint controllers remove the pod from service endpoints before listeners close.
- **Return Structured JSON Diagnostic Payloads**: Always return a machine-readable JSON body (`HealthResponse`) containing component-level statuses and error strings to enable automated diagnostics.

### Common Pitfalls & Anti-Patterns
- **Calling `db.Ping()` in `/livez`**: If the database slows down, all instances get killed and restarted simultaneously, compounding the database outage.
- **Returning HTTP 200 on Degraded Critical Checks**: Logging a failure internally but returning HTTP 200 OK to the orchestrator, preventing Kubernetes from removing unready pods from service rotation.
- **Heavy Database Queries in Readiness Probes**: Running table scans or non-indexed queries on `/readyz` probes every 5 seconds. Use lightweight connectivity checks (`SELECT 1` or `PingContext`) with strict timeouts.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/01_lifecycle/health_probes.go`.

### Core Types & Signatures

```go
package lifecycle

import (
	"context"
	"net/http"
	"time"
)

type HealthStatus string

const (
	StatusUp   HealthStatus = "UP"
	StatusDown HealthStatus = "DOWN"
)

type HealthCheckFunc func(ctx context.Context) error

type CheckDetail struct {
	Status HealthStatus `json:"status"`
	Error  string       `json:"error,omitempty"`
}

type HealthResponse struct {
	Status  HealthStatus           `json:"status"`
	Details map[string]CheckDetail `json:"details,omitempty"`
}

type ProbeHandler struct {
	// unexported fields: isStarted, isLive, isReady, checkTimeout, readinessChecks, livenessChecks
}

func NewProbeHandler(checkTimeout time.Duration) *ProbeHandler
func (h *ProbeHandler) SetStarted(started bool)
func (h *ProbeHandler) SetReady(ready bool)
func (h *ProbeHandler) SetLive(live bool)
func (h *ProbeHandler) RegisterReadinessCheck(name string, check HealthCheckFunc)
func (h *ProbeHandler) RegisterLivenessCheck(name string, check HealthCheckFunc)
func (h *ProbeHandler) StartupHandler() http.HandlerFunc
func (h *ProbeHandler) LivenessHandler() http.HandlerFunc
func (h *ProbeHandler) ReadinessHandler() http.HandlerFunc
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"patterns/01_lifecycle"
)

func main() {
	// 1. Initialize probe handler with 2-second timeout per check execution
	probes := lifecycle.NewProbeHandler(2 * time.Second)

	var db *sql.DB // Simulated DB instance

	// 2. Register dynamic readiness check for PostgreSQL
	probes.RegisterReadinessCheck("postgres_primary", func(ctx context.Context) error {
		if db == nil {
			return errors.New("database connection not initialized")
		}
		return db.PingContext(ctx)
	})

	// 3. Register liveness check (internal runtime check only)
	probes.RegisterLivenessCheck("runtime_deadlock", func(ctx context.Context) error {
		// Verify local process state
		return nil
	})

	// 4. Attach probe handlers to HTTP server mux
	mux := http.NewServeMux()
	mux.HandleFunc("/startupz", probes.StartupHandler())
	mux.HandleFunc("/livez", probes.LivenessHandler())
	mux.HandleFunc("/readyz", probes.ReadinessHandler())

	// 5. Simulate startup completion
	go func() {
		log.Println("Initializing application resources...")
		time.Sleep(1 * time.Second) // Simulate bootstrap/warmup
		probes.SetStarted(true)
		log.Println("Startup complete. Service is ready.")
	}()

	log.Println("Listening on :8080 with health probes enabled...")
	if err := http.ListenAndServe(":8080", mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server error: %v", err)
	}
}
```
