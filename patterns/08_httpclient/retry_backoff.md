# Exponential Backoff with Full Jitter and Context-Aware Retry

## Overview & Definition

Transient failures—such as momentary network blips, TCP resets, rate limits, and server restart spikes—are common in distributed cloud environments. Simply retrying requests immediately or with fixed sleep intervals creates synchronized traffic spikes ("thundering herds") that keep struggling services down.

The **Exponential Backoff with Full Jitter** pattern implements a resilient retry loop that:
1. Doubles the backoff delay with each consecutive failed attempt: $\text{Backoff} = \min(\text{MaxDelay}, \text{BaseDelay} \times 2^{\text{attempt}-1})$.
2. Applies **Full Jitter** by picking a uniform random duration in the range $[0, \text{Backoff}]$ to decorrelate concurrent client retries.
3. Classifies errors via `RetryableFunc` to immediately abort on permanent errors (e.g. HTTP 400, 401, 403, 422).
4. Respects `context.Context` cancellation and deadlines before each attempt and during backoff sleeps.

---

## Problem Statement

Naïve retry strategies in distributed systems cause catastrophic failure cascades:

* **Thundering Herd & Retry Storms:** When a downstream service experiences a 1-second outage, thousands of clients fail simultaneously. If all clients retry after a fixed delay (e.g. 100ms), they slam the recovering service with concentrated synchronized traffic waves, causing permanent downtime.
* **Retrying Non-Retryable Client Errors:** Retrying deterministic 4xx errors (e.g. Bad Request, Invalid Credentials) wastes client CPU and server capacity because the request will never succeed without client-side modification.
* **Context Leaks & Zombie Work:** Retries that do not check `ctx.Done()` continue sleeping and sending network requests even after the caller (or upstream HTTP client) has already aborted the request.

---

## Architectural Mechanism & Flow

```
                      ExecuteWithRetry(ctx, policy, fn)
                                     |
                                     v
                       +-----------------------------+
                       |   For attempt = 1 to Max    |
                       +-----------------------------+
                                     |
                                     v
                       +-----------------------------+
                       |    Check ctx.Done()         |
                       +-----------------------------+
                                /           \
                          [Canceled]       [Active]
                             /                \
                            v                  v
                   +------------------+  +-------------------------------+
                   | Return ctx.Err() |  | Execute fn(ctx, attempt)      |
                   +------------------+  +-------------------------------+
                                                       |
                                            +----------+----------+
                                            |                     |
                                            v                     v
                                      [Success / nil]      [Error returned]
                                            |                     |
                                            v           +---------+---------+
                                       [Return nil]     | Is error retryable|
                                                        | & attempt < Max?  |
                                                        +---------+---------+
                                                              /         \
                                                           [No]        [Yes]
                                                            /             \
                                          +----------------+       +-------+-------+
                                          | Abort & Return |       | Calculate     |
                                          | Last Error     |       | Jittered Delay|
                                          +----------------+       +-------+-------+
                                                                           |
                                                                           v
                                                           +-------------------------------+
                                                           | select {                      |
                                                           | case <-ctx.Done(): ...        |
                                                           | case <-time.After(jittered):  |
                                                           | }                             |
                                                           +-------------------------------+
                                                                           |
                                                                           v
                                                                   [Next Attempt Loop]
```

### Full Jitter Calculation
Following AWS Architecture research on backoff algorithms, **Full Jitter** achieves the highest throughput and lowest queue depth under contention:
$$\text{multiplier} = 2^{\text{attempt}-1}$$
$$\text{max\_backoff} = \min(\text{MaxDelay}, \text{BaseDelay} \times \text{multiplier})$$
$$\text{delay} = \text{rand}(0, \text{max\_backoff})$$

---

## Production Best Practices & Pitfalls

### Best Practices
* **Only Retry Idempotent Operations by Default:** Retrying non-idempotent `POST` or `PATCH` mutations without an `Idempotency-Key` can lead to duplicate payments, orders, or records.
* **Differentiate Transient vs. Permanent Errors:**
  * **Retryable:** Network timeouts, connection reset by peer, HTTP 429 (Too Many Requests), HTTP 502, 503, 504.
  * **Non-Retryable:** HTTP 400, 401, 403, 404, 422, JSON validation errors, business logic violations.
* **Cap the Maximum Delay:** Always specify a sensible `MaxDelay` (e.g., 2s – 5s) so backoff does not grow unbounded to minutes.

### Common Pitfalls
* **Using `time.Sleep` instead of Context-Aware Timers:** Using `time.Sleep(delay)` prevents immediate termination when the parent request context is canceled, holding worker goroutines captive.
* **Deterministic Exponential Backoff without Jitter:** Exponential backoff alone spreads retries over time, but clients that start at the same moment still retry in synchronized lockstep. Jitter is required to break synchronization.

---

## Code Walkthrough & Usage

### 1. Implementation (`retry_backoff.go`)

```go
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// RetryPolicy defines rules and backoff parameters.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryPolicy returns standard production retry configuration.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    2 * time.Second,
	}
}

// CalculateBackoffWithJitter computes exponential backoff with Full Jitter.
func (p RetryPolicy) CalculateBackoffWithJitter(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	multiplier := math.Pow(2, float64(attempt-1))
	backoff := float64(p.BaseDelay) * multiplier

	if backoff > float64(p.MaxDelay) {
		backoff = float64(p.MaxDelay)
	}

	// Full jitter: random duration between [0, backoff]
	jittered := rand.Float64() * backoff
	return time.Duration(jittered)
}

// RetryableFunc is an operation that can be retried.
type RetryableFunc func(ctx context.Context, attempt int) (retryable bool, err error)

// ExecuteWithRetry executes fn respecting RetryPolicy and Context cancellation.
func ExecuteWithRetry(ctx context.Context, policy RetryPolicy, fn RetryableFunc) error {
	var lastErr error

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry aborted due to context cancellation: %w", ctx.Err())
		default:
		}

		retryable, err := fn(ctx, attempt)
		if err == nil {
			return nil
		}

		lastErr = err
		if !retryable || attempt >= policy.MaxAttempts {
			return fmt.Errorf("attempt %d failed (non-retryable or max attempts exhausted): %w", attempt, err)
		}

		backoff := policy.CalculateBackoffWithJitter(attempt)
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry aborted during backoff: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}

	return errors.Join(errors.New("all retry attempts exhausted"), lastErr)
}
```

### 2. HTTP Client Retry Usage

```go
func FetchUserProfile(ctx context.Context, client *http.Client, userID string) (*UserProfile, error) {
    policy := DefaultRetryPolicy()
    var profile UserProfile

    err := ExecuteWithRetry(ctx, policy, func(ctx context.Context, attempt int) (bool, error) {
        req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/users/"+userID, nil)
        resp, err := client.Do(req)
        if err != nil {
            // Network failure: retryable
            return true, err
        }
        defer resp.Body.Close()

        if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
            // Downstream overload: retryable
            return true, fmt.Errorf("server error: %d", resp.StatusCode)
        }

        if resp.StatusCode != http.StatusOK {
            // Client error (400, 404, etc.): permanent, do not retry
            return false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
        }

        return false, json.NewDecoder(resp.Body).Decode(&profile)
    })

    return &profile, err
}
```
