package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

// IPRateLimiter tracks per-client token bucket limits.
type IPRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientBucket
	rate    float64
	burst   float64
}

func NewIPRateLimiter(rate float64, burst float64) *IPRateLimiter {
	return &IPRateLimiter{
		clients: make(map[string]*clientBucket),
		rate:    rate,
		burst:   burst,
	}
}

func (l *IPRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.clients[ip]
	now := time.Now()
	if !ok {
		l.clients[ip] = &clientBucket{
			tokens:     l.burst - 1.0,
			lastRefill: now,
		}
		return true
	}

	// Refill
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.lastRefill = now
	bucket.tokens += elapsed * l.rate
	if bucket.tokens > l.burst {
		bucket.tokens = l.burst
	}

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// ExtractClientIP parses the real client IP from headers or RemoteAddr.
func ExtractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

// RateLimitMiddleware enforces per-client rate limits returning 429 Too Many Requests on violation.
func RateLimitMiddleware(limiter *IPRateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractClientIP(r)
			if !limiter.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "too_many_requests",
					"message": "Rate limit exceeded. Please retry after some time.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
