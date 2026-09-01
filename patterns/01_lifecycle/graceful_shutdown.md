# Graceful Shutdown Pattern

## 1. Overview & Concept
The **Graceful Shutdown** pattern ensures that an application service terminates in a controlled, orderly sequence when receiving an operating system termination signal (`SIGTERM`, `SIGINT`) or encountering a fatal shutdown condition. Rather than immediately terminating active execution threads, closing open network sockets mid-transmission, or abruptly halting running database transactions, graceful shutdown orchestrates an orderly multi-stage teardown.

In high-scale Go backend systems, graceful shutdown coordinates across HTTP/gRPC servers, background worker goroutines, message queue consumers, telemetry buffers, and persistence connection pools. The shutdown process follows a strict prioritized lifecycle bounded by a deterministic timeout context:
1. **Traffic Ingress Halting**: Stop accepting new HTTP/gRPC connections and fail readiness health probes so load balancers divert new traffic.
2. **In-Flight Request Draining**: Allow active HTTP requests and ongoing transaction executions to complete within a bounded grace period.
3. **Worker & Consumer Draining**: Signal background worker pools and event stream consumers (e.g., Kafka, RabbitMQ) to finish current message batches and commit state.
4. **Buffer & Telemetry Flushing**: Flush in-memory log buffers (slog, zap) and OpenTelemetry trace/metric batches to collectors.
5. **Persistence & Client Teardown**: Close database connection pools, cache clients, and network sockets in reverse dependency order.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without a structured, prioritized graceful shutdown mechanism, abrupt terminations (`SIGKILL`, immediate `os.Exit(0)`, unhandled termination signals) cause severe production incidents:

- **In-Flight Data Corruption & Partial Mutations**: Requests interrupted mid-mutation leave databases in inconsistent states (e.g., deducting inventory without recording the payment or creating orphaned records).
- **Client-Side Connection Resets (`502 Bad Gateway` / `ECONNRESET`)**: Abruptly closing TCP listeners while load balancers are still routing requests causes sudden bursts of 502 errors on API gateways and client connection resets.
- **Message Broker Duplicate Delivery & Rebalance Storms**: Killing message consumers abruptly prevents offset commits or ACK frames. Brokers assume the worker died unexpectedly, triggering costly consumer group rebalances and causing duplicate message redelivery downstream.
- **Telemetry & Audit Blackouts**: Modern high-performance loggers and APM tracers buffer entries in memory before flushing in batches. Abrupt process termination drops buffered telemetry, leaving engineers blind during post-incident investigations.
- **Hanging / Zombie Deployments**: Without a bounded timeout, shutdown tasks blocked on broken database locks or deadlocked channels can hang forever, causing Kubernetes rolling deployments to stall until the cluster forcibly kills the pod via `SIGKILL`.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Teardown Sequence Diagram

```
OS / Orchestrator               SignalController         GracefulShutdownManager          HTTP Server / Workers        DB & Cache Pools
     |                                 |                            |                              |                         |
     |--- SIGTERM / SIGINT ----------->|                            |                              |                         |
     |                                 |--- Cancel ShutdownCtx ---->|                              |                         |
     |                                 |                            |-- (Priority 100) Shutdown -->|                         |
     |                                 |                            |   Stop listeners & drain reqs|                         |
     |                                 |                            |<-- Done / Drained -----------|                         |
     |                                 |                            |                              |                         |
     |                                 |                            |-- (Priority 50) Drain ------>| (Worker Pools)          |
     |                                 |                            |   Stop queue consumers       |                         |
     |                                 |                            |<-- Done / Committed ---------|                         |
     |                                 |                            |                              |                         |
     |                                 |                            |-- (Priority 10) Flush Buffers| (Logs / Traces)         |
     |                                 |                            |<-- Flushed ------------------|                         |
     |                                 |                            |                              |                         |
     |                                 |                            |-- (Priority 0) Close Pool ---------------------------->|
     |                                 |                            |<-- Pool Closed ----------------------------------------|
     |                                 |                            |                              |                         |
     |<-- Exit 0 ----------------------|<-- Complete All Hooks -----|                              |                         |
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                      GRACEFUL SHUTDOWN MANAGER                          |
 |                                                                         |
 |  Registered Hooks:                                                      |
 |  [Priority 100] -> HTTP Ingress & gRPC Servers (Stop accepting traffic) |
 |  [Priority  50] -> Background Goroutines & Queue Consumers (Drain work) |
 |  [Priority  20] -> Log & Trace Buffer Flushers (Slog / OTel)            |
 |  [Priority   0] -> DB Connections, Redis Pools & Network Drivers        |
 |                                                                         |
 |  Execution Flow:                                                        |
 |  1. Lock Mutex -> Verify !isClosed -> Set isClosed = true               |
 |  2. Stable Sort Hooks by Priority Descending                            |
 |  3. Derive Bounded Context: context.WithTimeout(parentCtx, timeout)     |
 |  4. Iterate Sorted Hooks:                                               |
 |     +--> Check ctx.Done() [If expired -> Return Timeout Error]          |
 |     +--> Execute hook.Fn(ctx)                                           |
 |     +--> On Error -> Accumulate via errors.Join                         |
 |  5. Return Aggregated Error Multierror                                  |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Align with Kubernetes Termination Grace Period**: Set the internal Go shutdown timeout strictly lower than Kubernetes `terminationGracePeriodSeconds` (e.g., if Kubernetes grace period is 30s, configure application shutdown timeout to 25s with 5s reserved for network/endpoint propagation).
- **Ensure Idempotency**: Protect `Shutdown()` execution with a mutex or atomic compare-and-swap so concurrent termination signals or multiple triggers do not re-execute cleanup tasks or panic on double-closed channels.
- **Accumulate Errors with `errors.Join`**: If one cleanup hook fails (e.g., Redis client timeout), do not abort the loop. Log the error and continue closing subsequent hooks (e.g., PostgreSQL and logs), returning an aggregated multi-error.
- **Derive Clean Contexts for Teardown**: Never pass an already-canceled parent context directly into cleanup functions (e.g., `db.Close(ctx)`). Always derive a fresh bounded timeout context from `context.Background()`.

### Common Pitfalls & Anti-Patterns
- **Closing Persistence Before Workers**: Shutting down database connection pools while background worker goroutines are still finishing in-flight tasks causes `sql: database is closed` errors in mid-flight operations.
- **Unbounded Channel or WaitGroup Blocking**: Calling `wg.Wait()` on worker pools without a select on `ctx.Done()`. If a worker hangs, the whole process blocks until killed forcibly.
- **Ignoring Hook Execution Errors**: Silently swallowing errors during database or queue teardown, hiding connection leak warnings and dirty flushes.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/01_lifecycle/graceful_shutdown.go`.

### Core Types & Signatures

```go
package lifecycle

import (
	"context"
	"time"
)

// CleanupFunc is a function executed during graceful shutdown.
type CleanupFunc func(ctx context.Context) error

// ShutdownHook holds a named cleanup task and its priority.
type ShutdownHook struct {
	Name     string
	Priority int // Higher priority runs first
	Fn       CleanupFunc
}

// GracefulShutdownManager manages the orderly teardown of application resources.
type GracefulShutdownManager struct {
	// unexported fields: mu, hooks, timeout, isClosed
}

func NewGracefulShutdownManager(timeout time.Duration) *GracefulShutdownManager
func (m *GracefulShutdownManager) Register(name string, priority int, fn CleanupFunc)
func (m *GracefulShutdownManager) Shutdown(parentCtx context.Context) error
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"patterns/01_lifecycle"
)

func main() {
	// 1. Initialize manager with a 15-second total cleanup budget
	shutdownMgr := lifecycle.NewGracefulShutdownManager(15 * time.Second)

	server := &http.Server{Addr: ":8080"}
	var db *sql.DB // Simulated database handle

	// 2. Register Priority 100: Stop accepting new ingress traffic
	shutdownMgr.Register("HTTP Server", 100, func(ctx context.Context) error {
		log.Println("Stopping HTTP server listener...")
		return server.Shutdown(ctx)
	})

	// 3. Register Priority 50: Drain background worker pools
	shutdownMgr.Register("Worker Pool", 50, func(ctx context.Context) error {
		log.Println("Draining background worker jobs...")
		// Worker drain logic here
		return nil
	})

	// 4. Register Priority 10: Flush logs and telemetry
	shutdownMgr.Register("Telemetry Buffers", 10, func(ctx context.Context) error {
		log.Println("Flushing telemetry spans and logs...")
		return nil
	})

	// 5. Register Priority 0: Close database and cache pools
	shutdownMgr.Register("Database Pool", 0, func(ctx context.Context) error {
		log.Println("Closing database connections...")
		if db != nil {
			return db.Close()
		}
		return nil
	})

	// 6. Execute graceful shutdown on termination
	ctx := context.Background()
	if err := shutdownMgr.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown encountered errors: %v", err)
	}
	log.Println("Application exited cleanly")
}
```
