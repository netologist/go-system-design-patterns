package concurrency_test

import (
	"testing"
	"time"

	concurrency "system-design-patterns/patterns/09_concurrency"
)

func TestTokenBucketRateLimiter_BurstAndRefill(t *testing.T) {
	// 10 tokens/sec, burst of 3
	limiter := concurrency.NewTokenBucketRateLimiter(10, 3)

	// First 3 should succeed immediately (burst)
	for i := range 3 {
		if !limiter.Allow() {
			t.Fatalf("expected token %d to be allowed", i+1)
		}
	}

	// 4th should be rejected immediately
	if limiter.Allow() {
		t.Fatal("expected 4th request to be rate limited")
	}

	// Wait 120ms -> should refill ~1 token (at 10 tokens/sec)
	time.Sleep(120 * time.Millisecond)

	if !limiter.Allow() {
		t.Fatal("expected token to be allowed after refill")
	}
}
