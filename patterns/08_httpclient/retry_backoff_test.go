package httpclient_test

import (
	"context"
	"errors"
	"testing"
	"time"

	httpclient "system-design-patterns/patterns/08_httpclient"
)

func TestExecuteWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	policy := httpclient.RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}

	attempts := 0
	err := httpclient.ExecuteWithRetry(context.Background(), policy, func(ctx context.Context, attempt int) (bool, error) {
		attempts++
		if attempt == 1 {
			return true, errors.New("503 service unavailable")
		}
		return false, nil // Success on attempt 2
	})

	if err != nil {
		t.Fatalf("expected success on second attempt, got: %v", err)
	}

	if attempts != 2 {
		t.Errorf("expected 2 attempts executed, got: %d", attempts)
	}
}

func TestExecuteWithRetry_PermanentErrorAbortsImmediately(t *testing.T) {
	policy := httpclient.DefaultRetryPolicy()

	attempts := 0
	err := httpclient.ExecuteWithRetry(context.Background(), policy, func(ctx context.Context, attempt int) (bool, error) {
		attempts++
		return false, errors.New("400 bad request (client error)")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 1 {
		t.Errorf("expected immediate stop on permanent error, but ran %d attempts", attempts)
	}
}

func TestCalculateBackoffWithJitter_Bounded(t *testing.T) {
	policy := httpclient.RetryPolicy{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	for i := range 10 {
		backoff := policy.CalculateBackoffWithJitter(i + 1)
		if backoff < 0 || backoff > 100*time.Millisecond {
			t.Errorf("backoff out of bounds: %v", backoff)
		}
	}
}
