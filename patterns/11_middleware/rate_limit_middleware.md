# Per-Client Rate Limiting Middleware

## 1. Overview & Concept

Uncontrolled client traffic—whether from denial-of-service (DoS) attempts, runaway scraper bots, or misconfigured mobile apps retrying in tight loops—can rapidly exhaust backend database connections and compute capacity.

The **Per-Client Rate Limiting Middleware** regulates inbound traffic on a per-client (IP-based) granularity using the **Token Bucket** algorithm. It allows legitimate bursty traffic up to a maximum burst capacity (`burst`), replenishes tokens continuously at a steady fill rate (`rate` tokens/sec), and immediately rejects excessive requests with **HTTP 429 Too Many Requests**, returning a standard `Retry-After` header and structured error payload.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without per-client rate limiting at the ingress layer:

* **Resource Depletion via API Abuse (Noisy Neighbors):** A single aggressive client or buggy frontend script can consume 90% of database connection pools, starving all other legitimate users across the system.
* **Denial of Service & Brute-Force Attacks:** Authentication endpoints (`/login`, `/reset-password`) without rate limiting are vulnerable to credential stuffing and dictionary attacks.
* **Memory Leaks from Unbounded Client Maps:** A naive in-memory map storing rate limit buckets for every seen IP address will grow indefinitely under millions of unique IPs, eventually triggering Go runtime Out-Of-Memory (OOM) crashes.
* **IP Spoofing Vulnerabilities:** Naively trusting client-supplied `X-Forwarded-For` headers without an edge proxy stripping untrusted upstream headers allows attackers to cycle fake IP addresses and bypass rate limits entirely.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The middleware extracts the client's IP address and checks for available tokens in the client's dedicated token bucket before delegating to downstream handlers.

```
                         Incoming HTTP Request
                                  │
                                  ▼
                     ┌──────────────────────────┐
                     │     ExtractClientIP      │
                     │ (XFF / X-Real-IP / Addr) │
                     └────────────┬─────────────┘
                                  │
                                  ▼
                     ┌──────────────────────────┐
                     │   IPRateLimiter.allow    │
                     │  - Lock mutex            │
                     │  - Refill tokens (dt*r)  │
                     │  - Check tokens >= 1.0   │
                     └────────────┬─────────────┘
                                  │
                   ┌──────────────┴──────────────┐
                   │                             │
            [Tokens >= 1.0]                [Tokens < 1.0]
                   │                             │
                   ▼                             ▼
           Deduct 1.0 token             Reject: HTTP 429
                   │                    - Header: Retry-After: 1
                   ▼                    - JSON error payload
          next.ServeHTTP(w, r)          - Return immediately
```

### Token Bucket Refill Math

```text
tokens_new = min(burst, tokens_old + elapsed_seconds * fill_rate)
if tokens_new >= 1.0:
    tokens_new -= 1.0
    allow_request()
else:
    reject_request(429)
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Secure Client IP Extraction & Reverse Proxies
When operating behind reverse proxies (Cloudflare, AWS ALB, NGINX), `r.RemoteAddr` reflects the proxy's IP. The `ExtractClientIP` helper inspects `X-Forwarded-For` (taking the left-most client IP) and `X-Real-IP`. **Production Warning:** Only trust `X-Forwarded-For` if your edge reverse proxy is configured to strip or overwrite untrusted incoming headers.

### 2. Lock Contention & Sharded Maps
The in-memory `IPRateLimiter` uses a `sync.Mutex`. Under tens of thousands of concurrent requests across many distinct IPs, a single global mutex can experience lock contention. For high-scale backends, consider sharding client maps across multiple mutexes (e.g. 16 or 32 hashed buckets) or utilizing Redis-based distributed sliding windows for multi-instance deployments.

### 3. Eviction Policy for Inactive Clients
In long-running backend processes, IPs that have not made requests in hours should be pruned periodically via a background goroutine or LRU cache to prevent unbounded heap memory consumption.

### 4. Downstream Header Contracts (`Retry-After`)
RFC 6585 specifies that HTTP 429 responses should include a `Retry-After` header indicating how many seconds the client must wait before retrying, helping compliant clients schedule their exponential backoff.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/11_middleware/rate_limit_middleware.go`)

```go
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

	// Refill tokens based on elapsed time
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
```

### 2. Composition in Production

```go
func SetupProtectedRoutes() http.Handler {
    // Allow 10 requests/second steady-state with a maximum burst of 20 requests
    limiter := middleware.NewIPRateLimiter(10.0, 20.0)

    chain := middleware.NewChain(
        middleware.RateLimitMiddleware(limiter),
    )

    mux := http.NewServeMux()
    mux.HandleFunc("/api/v1/search", searchHandler)
    return chain.Then(mux)
}
```
