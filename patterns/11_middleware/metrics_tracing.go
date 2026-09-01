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
