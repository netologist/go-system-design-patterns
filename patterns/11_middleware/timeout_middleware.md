# Request Timeout Middleware

## 1. Overview & Concept

In high-throughput microservice ecosystems, slow downstream operations (slow SQL queries, unindexed searches, deadlocked third-party webhooks) can tie up HTTP worker goroutines indefinitely. When thousands of requests stall waiting on hung I/O, server thread capacity is quickly exhausted, leading to cascading cluster-wide failures.

The **Request Timeout Middleware** enforces a strict processing deadline on incoming HTTP requests. It uses Go's standard `context.WithTimeout` to establish a cancellation deadline, executes downstream handlers in an isolated worker goroutine, and races handler completion against the context deadline. If the deadline expires before completion, the middleware immediately aborts the client request, returns a structured **HTTP 504 Gateway Timeout**, and signals context cancellation to abort in-flight database queries and network calls.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without an enforced request-level timeout middleware:

* **Goroutine & Connection Exhaustion (Thread Starvation):** If a downstream database query hangs for 60 seconds during a traffic surge of 500 RPS, the server spawns 30,000 concurrent goroutines within a single minute. Memory footprints explode, the Go runtime scheduler thrashes, and the container is killed by Kubernetes Out-Of-Memory (OOM) managers.
* **Cascading Upstream Timeouts:** When backend services stall, ingress reverse proxies (NGINX, Envoy, AWS ALB) hit their own 30-second timeouts and drop client connections. However, the backend server continues burning expensive CPU and database cycles computing responses that no client is listening for.
* **Silent Panic Swallowing:** In naive asynchronous timeout implementations (`go next.ServeHTTP()`), a panic occurring inside the spawned goroutine crashes the entire Go process because panics do not automatically bubble across goroutine boundaries unless captured via explicit `recover()` channels.
* **Concurrent Header Mutation Races:** If a timeout triggers and writes an HTTP 504 response to `http.ResponseWriter` while the background goroutine is still actively writing data, data races occur on the underlying TCP socket connection.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The middleware coordinates asynchronous handler execution with context deadline management and safe panic forwarding.

```
                      Client HTTP Request
                               │
                               ▼
            ┌──────────────────────────────────────┐
            │          TimeoutMiddleware           │
            │  ctx, cancel := WithTimeout(ctx, D)  │
            │  done := make(chan struct{})         │
            │  panicChan := make(chan any, 1)      │
            └──────────────────┬───────────────────┘
                               │
                    ┌──────────┴──────────┐
                    │                     │
                    ▼                     ▼
          [Main Goroutine]      [Spawned Worker Goroutine]
          select {              defer recover -> panicChan
          case rec := panicChan:next.ServeHTTP(w, r.WithContext(ctx))
          case <-done:          close(done)
          case <-ctx.Done():
          }
```

### Execution Scenarios

```text
Scenario A: Fast Completion (Within Deadline)
1. Worker executes handler -> finishes in 50ms -> closes `done`.
2. Main goroutine unblocks on `case <-done:`, returns 200 OK cleanly.

Scenario B: Timeout Exceeded (Deadline Expired)
1. Handler stalls (e.g. 5000ms query with 2000ms deadline).
2. Main goroutine triggers `case <-ctx.Done():`.
3. Sets HTTP 504 Gateway Timeout JSON payload to client.
4. Context cancellation propagates to DB driver/HTTP client to cancel query.

Scenario C: Panic in Worker Goroutine
1. Worker encounters panic -> deferred `recover()` catches error.
2. Worker sends panic value to `panicChan`.
3. Main goroutine re-panics in main thread -> caught by PanicRecoveryMiddleware.
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Panic Re-Propagation Across Goroutine Boundaries
A critical pitfall in concurrent Go programming is that `recover()` only catches panics within the *same* goroutine. The `TimeoutMiddleware` uses a buffered channel `panicChan := make(chan any, 1)` and a deferred `recover()` inside the worker goroutine to forward any panic to the main goroutine, where it is re-thrown via `panic(rec)` and handled by upstream recovery middleware.

### 2. Cooperative Cancellation Requirement
Context cancellation is **cooperative** in Go. Setting a timeout context will not forcefully kill a busy CPU-bound loop (e.g. infinite `for {}` loop or uncooperative crypto hashing). Application handlers and database drivers must actively listen to `ctx.Done()` (e.g., using `sql.DB.QueryContext` and `http.NewRequestWithContext`).

### 3. Response Writer Wrapping & Race Safety
When the timeout triggers, the main goroutine writes the 504 status code. If the worker goroutine attempts to write to `w` after timeout expiration, a race can occur. In production, handlers must check `ctx.Err() != nil` before writing response bodies or using a synchronized writer wrapper.

### 4. Memory Allocations per Request
Spawning a goroutine and allocating 2 channels per request has a negligible cost (~2KB stack + channel allocations). However, for ultra-high throughput endpoints (100k+ RPS), ensure timeout durations match SLA tiers and consider channel recycling if profile data indicates GC pressure.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/11_middleware/timeout_middleware.go`)

```go
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// TimeoutMiddleware applies a strict deadline to incoming request handlers.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			done := make(chan struct{})
			panicChan := make(chan any, 1)

			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			go func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicChan <- rec
					}
					close(done)
				}()
				next.ServeHTTP(wrapped, r.WithContext(ctx))
			}()

			select {
			case rec := <-panicChan:
				panic(rec) // Re-panic to allow recovery middleware to handle
			case <-done:
				// Successfully completed in time
				return
			case <-ctx.Done():
				// Deadline exceeded
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusGatewayTimeout)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "gateway_timeout",
					"message": "The request processing exceeded the configured timeout deadline",
				})
			}
		})
	}
}
```

### 2. Production Composition with Recovery Middleware

```go
func BuildAPIHandler() http.Handler {
    // Note: PanicRecovery MUST wrap TimeoutMiddleware so re-thrown panics are caught
    chain := middleware.NewChain(
        httpserver.PanicRecoveryMiddleware,
        middleware.TimeoutMiddleware(3*time.Second),
        middleware.AccessLoggerMiddleware(logger),
    )

    return chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Query database with cooperative cancellation
        err := db.QueryRowContext(r.Context(), "SELECT ...").Scan(...)
        if err != nil {
            if errors.Is(err, context.DeadlineExceeded) {
                // Timeout already fired; abort
                return
            }
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }))
}
```
