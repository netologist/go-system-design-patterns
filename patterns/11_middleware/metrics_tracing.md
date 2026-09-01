# Real-Time HTTP Metrics & Telemetry Middleware

## 1. Overview & Concept

To maintain high availability and meet stringent Service Level Objectives (SLOs), production backend services must track real-time telemetry on request throughput, HTTP error rates, and latency distributions across endpoints.

The **Real-Time HTTP Metrics Middleware** instruments incoming HTTP traffic at runtime, collecting high-frequency dimensional statistics:
1. **Request Throughput & Breakdown:** Granular request counters bucketed by HTTP Method, Route Path, and Response Status Code (e.g. `GET /api/v1/users 200`).
2. **Cumulative Latency Accounting:** Nanosecond-level latency accumulation using lock-free atomic integers (`atomic.Int64`).
3. **Success vs. Error Rate Tracking:** Real-time separation of 2xx/3xx successes vs 4xx/5xx failures to drive real-time alerting and error budget calculations.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without lightweight, low-overhead metrics collection:

* **Unmonitored Error Rate Spikes:** Outages caused by bad releases, database migrations, or third-party service failures remain undetected until customer support tickets escalate.
* **Latency Degradation Blindspots:** A gradual p99 latency increase from 20ms to 800ms goes unnoticed without cumulative or histogram-based latency telemetry.
* **Cardinality Explosion in Metrics Registries:** Storing raw URLs containing un-parameterized IDs (e.g. `/orders/18293749` instead of `/orders/:id`) creates millions of unique keys in the metrics map, quickly exhausting server heap memory and crashing Prometheus scrapers.
* **Lock Contention on Telemetry Hot-Paths:** Using heavy global mutexes for every request counter creates CPU bottlenecks on multi-core servers under 100k+ RPS.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The middleware utilizes a hybrid synchronization strategy: an `RWMutex` with double-checked locking to lazily register endpoint keys, paired with lock-free `atomic.Int64` counters for high-speed increments.

```
                      Client HTTP Request
                               │
                               ▼
             ┌────────────────────────────────────┐
             │         MetricsMiddleware          │
             │  start := time.Now()               │
             │  wrapped := &responseWriterWrapper │
             └─────────────────┬──────────────────┘
                               │
                               ▼
             ┌────────────────────────────────────┐
             │         Downstream Handler         │
             │  Executes business logic           │
             │  Emits status code & response body │
             └─────────────────┬──────────────────┘
                               │
                               ▼
             ┌────────────────────────────────────┐
             │       metrics.Record(...)          │
             │  - Key: "METHOD /path STATUS"      │
             │  - Atomic Add to requestCount[key] │
             │  - Atomic Add to totalLatency      │
             │  - Atomic Add to Success / Error   │
             └─────────────────┬──────────────────┘
                               │
                               ▼
                 Exported to Prometheus / Datadog
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Lock-Free Hot Paths via `sync/atomic`
Once a route key (e.g. `POST /api/v1/orders 201`) is initialized in the map, all subsequent metric updates execute lock-free using `atomic.Int64.Add(1)`. The read-lock (`m.mu.RLock()`) guarantees safe concurrent map reads without blocking concurrent request goroutines.

### 2. Guarding Against Metric Cardinality Explosion
If clients request dynamic URLs (e.g., `/users/UUID-1234`), passing raw `r.URL.Path` directly into the metrics key will produce unbounded map entries. In production routers (such as `chi`, `gin`, or Go 1.22+ `http.ServeMux`), extract the route pattern template (`r.Pattern` or route definition) rather than raw unparsed URL paths.

### 3. Separation of Client Errors (4xx) vs Server Failures (5xx)
While status codes $\ge 400$ are categorized as errors, SLO monitoring typically distinguishes 4xx (client-side validation errors, 401s, 404s) from 5xx (server faults, 500s, 503s, 504s) to ensure automated alerts only page on-call engineers for actionable infrastructure issues.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/11_middleware/metrics_tracing.go`)

```go
package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPMetrics holds runtime statistics for HTTP traffic.
type HTTPMetrics struct {
	mu           sync.RWMutex
	requestCount map[string]*atomic.Int64 // key: "METHOD /path STATUS"
	totalLatency atomic.Int64             // Nanoseconds
	totalSuccess atomic.Int64
	totalErrors  atomic.Int64
}

func NewHTTPMetrics() *HTTPMetrics {
	return &HTTPMetrics{
		requestCount: make(map[string]*atomic.Int64),
	}
}

func (m *HTTPMetrics) Record(method, path string, status int, duration time.Duration) {
	key := fmt.Sprintf("%s %s %d", method, path, status)

	m.mu.RLock()
	counter, ok := m.requestCount[key]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		counter, ok = m.requestCount[key]
		if !ok {
			counter = &atomic.Int64{}
			m.requestCount[key] = counter
		}
		m.mu.Unlock()
	}

	counter.Add(1)
	m.totalLatency.Add(duration.Nanoseconds())

	if status >= 400 {
		m.totalErrors.Add(1)
	} else {
		m.totalSuccess.Add(1)
	}
}

func (m *HTTPMetrics) TotalRequests() int64 {
	return m.totalSuccess.Load() + m.totalErrors.Load()
}

func (m *HTTPMetrics) TotalErrors() int64 {
	return m.totalErrors.Load()
}

// MetricsMiddleware instruments incoming HTTP requests and updates metrics.
func MetricsMiddleware(metrics *HTTPMetrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			if metrics != nil {
				metrics.Record(r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
			}
		})
	}
}
```

### 2. Prometheus / Health Export Endpoint Usage

```go
func SetupMetricsEndpoint(metrics *middleware.HTTPMetrics) http.Handler {
    mux := http.NewServeMux()

    mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        total := metrics.TotalRequests()
        errors := metrics.TotalErrors()
        
        json.NewEncoder(w).Encode(map[string]any{
            "total_requests": total,
            "total_errors":   errors,
            "error_rate":     float64(errors) / float64(max(total, 1)),
        })
    })

    return mux
}
```
