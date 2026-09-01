package database

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

var (
	ErrDeadlockDetected      = errors.New("pq: deadlock detected (SQLSTATE 40P01)")
	ErrSerializationFailure  = errors.New("pq: could not serialize access (SQLSTATE 40001)")
	ErrMaxDeadlockRetries    = errors.New("transaction failed: maximum deadlock retries exceeded")
)

// IsDeadlockError checks if the error is a transient concurrency conflict that should be retried.
func IsDeadlockError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrDeadlockDetected) || errors.Is(err, ErrSerializationFailure) {
		return true
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "deadlock") || strings.Contains(errStr, "could not serialize")
}

// ExecuteWithDeadlockRetry executes a database transaction with automatic retry on deadlocks.
func ExecuteWithDeadlockRetry(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func(ctx context.Context) error) error {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if baseDelay <= 0 {
		baseDelay = 20 * time.Millisecond
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		if !IsDeadlockError(err) {
			// Permanent error -> Return immediately without retry
			return err
		}

		if attempt >= maxRetries {
			break
		}

		// Randomized backoff to break the deadlock cycle between competing transactions
		delay := baseDelay * time.Duration(1<<uint(attempt-1))
		jitter := time.Duration(rand.Int64N(int64(delay)))
		totalWait := delay + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(totalWait):
		}
	}

	return fmt.Errorf("%w: %v", ErrMaxDeadlockRetries, lastErr)
}
