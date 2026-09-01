# Graceful Startup Pattern

## 1. Overview & Concept
The **Graceful Startup** pattern guarantees that an application service validates all prerequisite infrastructure dependencies (such as primary databases, cache clusters, configuration servers, and identity providers) and executes required warmup routines *before* binding network listeners and advertising itself as ready to accept external traffic.

Rather than opening listening ports prematurely and failing incoming user requests due to uninitialized dependencies, the service executes concurrent pre-flight validation checks with deterministic timeouts. By categorizing dependencies into **critical** (mandatory for service operation) and **non-critical** (optional or degradable dependencies), the application can fail fast on fatal configuration or connectivity errors while gracefully tolerating degraded peripheral systems.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Starting network listeners before validating dependencies causes severe production disruptions:

- **Thundering Herd Cascades on Boot**: When an orchestrator (Kubernetes) deploys a new service replica, binding to port `:8080` immediately signals readiness. If the database connection pool has not established minimum idle connections or schema migrations are lagging, incoming traffic causes an avalanche of connection timeouts and `500 Internal Server Error` spikes.
- **Silent Degradation & Delayed Crash Loops**: A service may boot without throwing an immediate error, only to crash 30 seconds later on the first user request because an uninitialized Redis client or messaging producer triggers a nil-pointer dereference.
- **Poisoning Rolling Deployments**: During a rolling update, if new pods start without validating database connectivity, Kubernetes terminates healthy existing pods and replaces them with broken new pods, escalating an isolated outage into a total cluster outage.
- **Serialized Startup Bottlenecks**: Performing dependency checks sequentially (e.g., waiting 3s for DB ping, then 2s for Redis, then 2s for Auth) results in slow bootstrap times, degrading horizontal pod autoscaling (HPA) responsiveness.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Startup Sequence Diagram

```
Application Main / Bootstrap           StartupValidator                 PostgreSQL (Critical)          Redis (Non-Critical)
        |                                     |                                    |                             |
        |--- AddDependency("Postgres", true)->|                                    |                             |
        |--- AddDependency("Redis", false) -->|                                    |                             |
        |                                     |                                    |                             |
        |--- ValidateAll(ctx) --------------->|                                    |                             |
        |                                     |-- Concurrent Ping (Goroutine) ---->|                             |
        |                                     |-- Concurrent Ping (Goroutine) ---------------------------------->|
        |                                     |                                    |                             |
        |                                     |<-- Ping OK ------------------------|                             |
        |                                     |<-- Ping Failed (Timeout/Error) ----------------------------------|
        |                                     |                                                                  |
        |                                     |-- Check Critical Flags:                                          |
        |                                     |   - Postgres (Critical) -> OK                                    |
        |                                     |   - Redis (Non-Critical) -> Log Warning (Ignored for startup exit)|
        |                                     |                                                                  |
        |<-- Success (No Critical Errors) ----|                                                                  |
        |                                     |                                                                  |
        |--- Bind Port :8080 & Serve Traffic                                                                     |
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                          STARTUP VALIDATOR                              |
 |                                                                         |
 |  Registered Dependency Checks:                                          |
 |  +-> [Postgres DB]  | Critical: true  | Timeout: 2s | Pinger: db.Ping   |
 |  +-> [Auth Service] | Critical: true  | Timeout: 2s | Pinger: rpc.Ping  |
 |  +-> [Redis Cache]  | Critical: false | Timeout: 1s | Pinger: rdb.Ping  |
 |                                                                         |
 |  Validation Execution:                                                  |
 |  1. Derive Global Timeout Context: context.WithTimeout(ctx, totalLimit) |
 |  2. Fan-out Goroutines per Dependency Check                             |
 |  3. Enforce Per-Dependency Timeout vs Global Budget                     |
 |  4. Collect Results over Buffered Channel (len = num_deps)              |
 |  5. Sync via sync.WaitGroup -> Close Channel                            |
 |  6. Filter & Aggregate Errors:                                          |
 |     +--> Critical Failure -> Append to criticalErrors (errors.Join)     |
 |     +--> Non-Critical Failure -> Log as Non-Fatal Warning               |
 |  7. Return aggregated error (Blocks startup if criticalErrors > 0)      |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Concurrent Fan-Out**: Always execute pre-flight connectivity checks in parallel goroutines so total startup check latency equals $\max(\text{check durations})$ rather than $\sum(\text{check durations})$.
- **Explicit Per-Dependency Timeouts**: Always bound individual dependency pings with dedicated timeouts to prevent a single hanging TCP socket from consuming the entire startup budget.
- **Classify Criticality Conservatively**: Core databases and identity providers must be marked `Critical: true`. Optional caching layers, non-essential analytics pipelines, or secondary sinks should be marked `Critical: false` to allow degraded operation.
- **Pre-Warm Connection Pools & Caches**: During the validation phase, initialize minimum idle connections (`db.SetMinIdleConns`) and pre-fetch required static configuration/caches before accepting live requests.

### Common Pitfalls & Anti-Patterns
- **Binding Listeners Before Validation Completes**: Spawning `http.ListenAndServe()` in a separate goroutine before `ValidateAll()` has returned successfully allows load balancers to route live traffic to an unverified instance.
- **Failing Startup on Non-Critical Peripherals**: Terminating a service boot because an optional metric push-gateway or non-essential cache node is offline, needlessly degrading overall system availability.
- **Unbounded Contexts on Startup**: Relying on default client timeouts (which may be infinite), causing pods to hang indefinitely during container initialization until Kubernetes kills them.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/01_lifecycle/graceful_startup.go`.

### Core Types & Signatures

```go
package lifecycle

import (
	"context"
	"time"
)

// Pinger defines an interface for checking connectivity to a dependency.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Dependency represents a named external service that must be validated at startup.
type Dependency struct {
	Name     string
	Critical bool
	Pinger   Pinger
	Timeout  time.Duration
}

// StartupValidator coordinates validating all dependencies before accepting traffic.
type StartupValidator struct {
	// unexported fields: dependencies, totalTimeout
}

func NewStartupValidator(totalTimeout time.Duration) *StartupValidator
func (v *StartupValidator) AddDependency(name string, critical bool, timeout time.Duration, pinger Pinger)
func (v *StartupValidator) ValidateAll(parentCtx context.Context) error
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"patterns/01_lifecycle"
)

// Mock database adapter implementing lifecycle.Pinger
type PostgresPinger struct{}

func (p *PostgresPinger) Ping(ctx context.Context) error {
	// Real implementation: return db.PingContext(ctx)
	return nil
}

// Mock cache adapter implementing lifecycle.Pinger
type RedisPinger struct{}

func (r *RedisPinger) Ping(ctx context.Context) error {
	// Real implementation: return rdb.Ping(ctx).Err()
	return errors.New("redis cluster failover in progress")
}

func main() {
	// 1. Create a validator with a 5-second total startup deadline
	validator := lifecycle.NewStartupValidator(5 * time.Second)

	// 2. Register Critical dependencies (must succeed to start)
	validator.AddDependency("PostgreSQL", true, 2*time.Second, &PostgresPinger{})

	// 3. Register Non-Critical dependencies (tolerates startup degradation)
	validator.AddDependency("RedisCache", false, 1*time.Second, &RedisPinger{})

	// 4. Validate all dependencies concurrently
	ctx := context.Background()
	log.Println("Starting pre-flight dependency validation...")
	if err := validator.ValidateAll(ctx); err != nil {
		log.Fatalf("Fatal startup validation failed: %v", err)
	}

	// 5. Open network listener only after validation passes
	log.Println("All critical dependencies verified. Binding HTTP port :8080...")
	server := &http.Server{Addr: ":8080"}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
```
