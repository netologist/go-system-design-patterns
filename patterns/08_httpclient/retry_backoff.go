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

// CalculateBackoffWithJitter computes exponential backoff with Full Jitter:
// Sleep = rand(0, min(MaxDelay, BaseDelay * 2^attempt))
func (p RetryPolicy) CalculateBackoffWithJitter(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// 2^(attempt-1)
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
// It returns true if the error is temporary/retryable, false if permanent.
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
