# Overload Protection Pattern (Global Concurrency Limiter)

## 1. Overview & Concept
The **Overload Protection Pattern** protects backend systems from catastrophic throughput collapse, high-latency queuing, and out-of-memory (OOM) crashes by enforcing an upper bound on concurrent in-flight requests processed by the server. When server concurrency saturates, rather than allowing unbound request accumulation in kernel socket buffers or internal memory queues, this pattern **fast-fails** excess incoming requests with HTTP `503 Service Unavailable` and a `Retry-After` header.

By capping concurrency at the maximum safe operational threshold, the server guarantees predictable latency and maximum **goodput** (requests completed successfully within SLA) for in-flight traffic.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
In high-throughput microservices, incoming request spikes frequently exceed hardware capacity (CPU, memory, database connection pools, file descriptors).

### Failure Scenarios Without This Pattern
- **Catastrophic Latency Collapse:** As concurrency increases beyond saturation, goroutine context switching, CPU cache thrashing, and memory allocations skyrocket. Response latency degrades from 20ms to 30+ seconds, causing upstream callers to time out while the server wastes CPU on dead requests.
- **Out-of-Memory (OOM) Panics:** Each accepted HTTP request allocates buffers, headers, and request structures. Under a massive traffic spike (e.g., 50,000 concurrent requests), total heap memory exceeds container limits, triggering Kubernetes OOM-kill (`SIGKILL`).
- **Connection Pool Exhaustion:** Unbounded incoming requests exhaust database connection pools and Redis sockets, causing cascading failures across all endpoints sharing the pool.
- **Dogpiling After Transient Outages:** When an upstream dependency recovers, queued requests surge simultaneously into downstream services, crashing them immediately.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `OverloadProtectionMiddleware` uses atomic integer counters (`sync/atomic.Int64`) to track active concurrent requests with zero lock contention:

```
          [ Incoming HTTP Request ]
                      │
                      ▼
       ┌──────────────────────────────┐
       │ OverloadProtectionMiddleware │
       └──────────────┬───────────────┘
                      │
        activeRequests.Add(1)
                      │
       ┌──────────────┴──────────────┐
       │ activeRequests > maxLimit?  │
       └──────────────┬──────────────┘
              │                 │
             [Yes]             [No]
              │                 │
              ▼                 ▼
   activeRequests.Add(-1)   [ Execute next.ServeHTTP(w, r) ]
              │                 │
              ▼                 ▼
   [ Return HTTP 503 ]      [ Return HTTP 200 OK ]
   [ Retry-After: 2s ]          │
                                ▼
                       activeRequests.Add(-1)
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Zero-Lock Atomic Fast-Path:** Use `atomic.Int64.Add(1)` to ensure concurrency tracking introduces sub-microsecond latency and zero mutex lock contention on high-throughput endpoints.
- **Provide Standard `Retry-After` Headers:** Include a `Retry-After: <seconds>` header in 503 responses so well-behaved HTTP clients and SDKs back off rather than hammering the server in a tight retry loop.
- **Exempt Health & Liveness Probes:** Never reject Kubernetes `/healthz` or `/readyz` probes during overload. Dropping liveness probes causes Kubernetes to terminate healthy overloaded pods, worsening the cascade.
- **Tune Limit to Little's Law:** Calculate `maxActiveRequests` using Little's Law: $L = \lambda \times W$, where $\lambda$ is maximum sustainable throughput (req/sec) and $W$ is target average latency (seconds).
- **Track Metrics for SRE Alerting:** Export `http_requests_overload_rejected_total` to Prometheus to trigger auto-scaling alerts.

### Pitfalls to Avoid
- **Unbounded Queueing Instead of Dropping:** Queuing excess requests in memory introduces latency without increasing capacity. Rejecting excess load fast preserves goodput.
- **Missing `defer activeRequests.Add(-1)`:** If downstream handlers panic and recover without decrementing the atomic counter, the capacity pool permanently leaks until the server rejects all traffic.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/20_reliability/overload_protection.go`
```go
package reliability

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// OverloadProtectionMiddleware limits maximum concurrent active requests across the whole server.
// When capacity is exceeded, it returns controlled 503 Service Unavailable rather than collapsing the process.
func OverloadProtectionMiddleware(maxActiveRequests int64) func(http.Handler) http.Handler {
	if maxActiveRequests <= 0 {
		maxActiveRequests = 1000
	}

	var activeRequests atomic.Int64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := activeRequests.Add(1)
			defer activeRequests.Add(-1)

			if current > maxActiveRequests {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "service_unavailable",
					"message": "Server capacity temporarily saturated. Please retry shortly.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

### Production HTTP Server Integration Example
```go
func StartHardenedHTTPServer(handler http.Handler) *http.Server {
    // Wrap application router with 500 concurrent in-flight limit
    overloadProtected := reliability.OverloadProtectionMiddleware(500)(handler)

    return &http.Server{
        Addr:              ":8080",
        Handler:           overloadProtected,
        ReadHeaderTimeout: 3 * time.Second,
        WriteTimeout:      10 * time.Second,
        IdleTimeout:       30 * time.Second,
    }
}
```
