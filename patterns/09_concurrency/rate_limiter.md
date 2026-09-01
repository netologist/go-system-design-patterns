# Token Bucket Rate Limiter Pattern

## Overview & Definition

Rate limiting controls the frequency of operations, protecting systems from denial-of-service, API quota exhaustion, and CPU overload.

The **Token Bucket Rate Limiter** pattern models rate limiting using a virtual bucket:
* **Rate ($r$):** The number of tokens continuously deposited into the bucket per second.
* **Burst Capacity ($b$):** The maximum number of tokens the bucket can hold.
* **Allow():** Non-blocking check that consumes 1 token if available; returns `false` if the bucket is empty.
* **Wait(ctx):** Blocking call that yields until a token is replenished or the context deadline expires.

Tokens are lazily replenished based on elapsed time ($\Delta t \times r$) during each operation, eliminating the need for background timer goroutines.

---

## Problem Statement

Without effective rate limiting:

* **Denial of Service & API Scrapes:** Malicious bots or misconfigured clients can hammer public endpoints at thousands of requests per second, starving legitimate users.
* **Downstream Third-Party Quota Breaches:** When integrating with external APIs with strict limits (e.g. Stripe, Twilio), unthrottled bursts lead to HTTP 429 rate limit penalties or account suspension.
* **CPU & Memory Spikes on Bursty Traffic:** Sudden traffic spikes cause queue buildup, GC pauses, and memory pressure.

---

## Architectural Mechanism & Flow

```
                      Client Request Arrival
                                |
                                v
                   [Acquire Mutex Lock mu]
                                |
                                v
                   [Lazy Refill Calculation]
          Tokens += (now - lastRefill) * rate (cap at burst)
          lastRefill = now
                                |
                                v
                     [Evaluate Available Tokens]
                               /           \
                     [Tokens >= 1.0]     [Tokens < 1.0]
                           /                   \
                          v                     v
                [Tokens -= 1.0]          [Release Lock mu]
                [Release Lock mu]        [Return false / Wait]
                [Return true (Allow)]
```

### Lazy Refill Calculation
Instead of spawning a background ticker goroutine that ticks every millisecond, tokens are calculated dynamically on each access:
$$\Delta t = \text{now} - \text{lastRefill}$$
$$\text{tokens} = \min(\text{burst}, \text{tokens} + (\Delta t \times \text{rate}))$$
$$\text{lastRefill} = \text{now}$$

---

## Production Best Practices & Pitfalls

### Best Practices
* **Use Lazy Refill:** Calculating token addition lazily upon access scales to millions of active keys/users with zero background goroutine overhead.
* **Support Burst Capacity:** Set `burst` higher than `rate` (e.g. `rate=10/s`, `burst=20`) to allow legitimate users to load web pages with multiple parallel asset requests without immediate throttling.
* **Provide `Retry-After` Headers:** When `Allow()` returns `false` in an HTTP context, return HTTP 429 with a calculated `Retry-After` header indicating when the next token will be available.

### Common Pitfalls
* **Spawning Tickers Per Limiter:** Creating a `time.Ticker` goroutine for every user/client limiter will leak thousands of goroutines and destroy scheduler performance.
* **High Lock Contention on Single Global Limiter:** In high-throughput systems (>100k RPS), a single mutex on a global rate limiter causes CPU cache bouncing. Shard limiters by CPU core or client ID.

---

## Code Walkthrough & Usage

### 1. Implementation (`rate_limiter.go`)

```go
package concurrency

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

// TokenBucketRateLimiter controls execution rate using the Token Bucket algorithm.
type TokenBucketRateLimiter struct {
	mu         sync.Mutex
	rate       float64 // Tokens added per second
	burst      float64 // Maximum bucket capacity
	tokens     float64
	lastRefill time.Time
}

// NewTokenBucketRateLimiter creates a rate limiter with given rate (per second) and burst limit.
func NewTokenBucketRateLimiter(rate float64, burst float64) *TokenBucketRateLimiter {
	if rate <= 0 {
		rate = 10
	}
	if burst <= 0 {
		burst = rate
	}

	return &TokenBucketRateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     burst,
		lastRefill: time.Now(),
	}
}

// Allow checks if 1 token is available without blocking.
func (r *TokenBucketRateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillLocked()

	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

// Wait blocks until a token becomes available or ctx is canceled.
func (r *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if r.Allow() {
			return nil
		}

		// Sleep briefly before polling
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (r *TokenBucketRateLimiter) refillLocked() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.lastRefill = now

	r.tokens += elapsed * r.rate
	if r.tokens > r.burst {
		r.tokens = r.burst
	}
}
```

### 2. Outbound Client Throttling Example

```go
type ThirdPartySMSClient struct {
    limiter *TokenBucketRateLimiter
}

func (c *ThirdPartySMSClient) SendSMS(ctx context.Context, to, msg string) error {
    // Block until token available within context deadline
    if err := c.limiter.Wait(ctx); err != nil {
        return fmt.Errorf("rate limit wait aborted: %w", err)
    }

    return c.dispatchToGateway(ctx, to, msg)
}
```
