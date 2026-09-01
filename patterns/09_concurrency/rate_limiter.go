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
	mu          sync.Mutex
	rate        float64 // Tokens added per second
	burst       float64 // Maximum bucket capacity
	tokens      float64
	lastRefill  time.Time
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
