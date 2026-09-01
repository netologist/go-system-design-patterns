# Retry Budget

## Overview & Definition

While retries with exponential backoff resolve transient isolated errors, widespread system degradation causes standard retry policies to multiply overall traffic. If 1,000 clients each retry 3 times against a struggling service, total traffic spikes to 4,000 requests, driving the service deeper into collapse.

The **Retry Budget** pattern (pioneered by Google SRE and Finagle/Envoy) dynamically caps the total volume of retries as a strict percentage of successful/regular requests (typically 10%–20%). It implements a token bucket mechanism:
1. Every regular, non-retry request deposits a fraction of a retry token (`ratio`, e.g. 0.10).
2. Every retry attempt consumes exactly 1.0 token from the budget (`TryAcquireRetry`).
3. A baseline replenishment rate (`minRetriesSec`) ensures low-traffic services can still perform necessary retries.

If the retry budget is exhausted, further retries are rejected immediately, failing fast rather than exacerbating downstream outages.

---

## Problem Statement

Unconstrained retries during outages create the **Retry Storm (Amplification) Anti-Pattern**:

* **Traffic Multiplication During Degradation:** When a backend database experiences lock contention and fails 50% of requests, clients retrying 3 times increase total request volume by 150%, turning a recoverable incident into an extended total outage.
* **Cascading Upstream Saturation:** When downstream service latency increases, retries consume thread pools, socket buffers, and memory on intermediate proxy and API gateway layers.
* **Loss of Backpressure:** Retrying without budget awareness hides degradation until all service tiers exhaust connection limits.

---

## Architectural Mechanism & Flow

```
                                  Incoming Request Flow
                                            |
                                            v
                             +-----------------------------+
                             |   Is it a regular request?  |
                             +-----------------------------+
                                      /            \
                                  [Yes]            [No / Retry Attempt]
                                   /                \
                                  v                  v
                   +------------------------+  +-------------------------------+
                   | budget.RecordRequest() |  | budget.TryAcquireRetry()      |
                   | Tokens += ratio (0.10) |  +-------------------------------+
                   +------------------------+             /                 \
                                                   [Tokens >= 1.0]     [Tokens < 1.0]
                                                         /                   \
                                                        v                     v
                                            +---------------------+  +-----------------+
                                            | Consume 1.0 Token   |  | Reject Retry    |
                                            | Allow Retry Call    |  | Fail Fast       |
                                            +---------------------+  +-----------------+
```

### Token Bucket Math
* **Token Accumulation:** $\text{Tokens}_{\text{new}} = \min(\text{MaxTokens}, \text{Tokens}_{\text{current}} + \text{ratio} + (\Delta t \times \text{MinRetriesSec}))$
* **Token Consumption:** If $\text{Tokens} \ge 1.0$, subtract $1.0$ and permit retry. Otherwise, deny retry.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Integrate at the HTTP Client Wrapper Layer:** Wrap `http.Client.Do` or the retry loop with `budget.RecordRequest()` on initial calls and `budget.TryAcquireRetry()` before secondary attempts.
* **Tune Ratio to 10% – 20%:** A 10% budget means the service will never emit more than 1.1x its baseline request rate during an outage, preventing downstream collapse while still recovering isolated errors.
* **Include a Minimum Baseline (`minRetriesSec`):** On quiet background workers or low-traffic services (e.g. 1 req/min), fractional token earning might take minutes to accumulate 1 token. A baseline of 5–10 retries/second ensures healthy low-traffic operation.

### Common Pitfalls
* **Counting Retries as Regular Requests:** Calling `RecordRequest()` on retry attempts artificially inflates tokens, defeating the budget during outages.
* **Single Shared Budget for Heterogeneous Backends:** Using one global budget across Payment, Auth, and Analytics backends allows an outage in Analytics to consume all tokens, starving critical Payment retries. Maintain a budget per downstream destination.

---

## Code Walkthrough & Usage

### 1. Implementation (`retry_budget.go`)

```go
package httpclient

import (
	"sync"
	"time"
)

// RetryBudget limits retry volume relative to regular requests (e.g. max 10% retries).
// This prevents cascading failures / retry storms during major outages.
type RetryBudget struct {
	mu            sync.Mutex
	ratio         float64 // e.g. 0.10 for 10% retry budget
	tokens        float64
	maxTokens     float64
	minRetriesSec float64
	lastUpdated   time.Time
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
```

### 2. Integration with Retry Loop

```go
type ResilientClient struct {
    client *http.Client
    budget *RetryBudget
    policy RetryPolicy
}

func (c *ResilientClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
    // Record regular request
    c.budget.RecordRequest()

    for attempt := 1; attempt <= c.policy.MaxAttempts; attempt++ {
        if attempt > 1 {
            // Check budget before attempting retry
            if !c.budget.TryAcquireRetry() {
                return nil, errors.New("retry rejected: retry budget exhausted")
            }
        }

        resp, err := c.client.Do(req.Clone(ctx))
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }

        if attempt < c.policy.MaxAttempts {
            time.Sleep(c.policy.CalculateBackoffWithJitter(attempt))
        }
    }

    return nil, errors.New("all retries exhausted")
}
```
