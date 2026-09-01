package httpclient

import (
	"sync"
	"time"
)

// RetryBudget limits retry volume relative to regular requests (e.g. max 10% retries).
// This prevents cascading failures / retry storms during major outages.
type RetryBudget struct {
	mu           sync.Mutex
	ratio        float64 // e.g. 0.10 for 10% retry budget
	tokens       float64
	maxTokens    float64
	minRetriesSec float64
	lastUpdated  time.Time
}

// NewRetryBudget creates a budget with a target retry ratio (e.g. 0.10) and minimum baseline per second.
func NewRetryBudget(ratio float64, maxTokens float64, minRetriesSec float64) *RetryBudget {
	if ratio <= 0 || ratio > 1.0 {
		ratio = 0.10
	}
	if maxTokens <= 0 {
		maxTokens = 100
	}
	if minRetriesSec <= 0 {
		minRetriesSec = 10
	}

	return &RetryBudget{
		ratio:         ratio,
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		minRetriesSec: minRetriesSec,
		lastUpdated:   time.Now(),
	}
}

// RecordRequest is called on every regular (non-retry) request to earn retry budget tokens.
func (b *RetryBudget) RecordRequest() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.replenishLocked()

	// Every request deposits `ratio` fraction of a token
	b.tokens += b.ratio
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
}

// TryAcquireRetry consumes 1 token from the budget if available.
func (b *RetryBudget) TryAcquireRetry() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.replenishLocked()

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}

	return false
}

func (b *RetryBudget) replenishLocked() {
	now := time.Now()
	elapsed := now.Sub(b.lastUpdated).Seconds()
	b.lastUpdated = now

	// Baseline replenishment
	b.tokens += elapsed * b.minRetriesSec
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
}
