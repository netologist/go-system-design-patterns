package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	database "system-design-patterns/patterns/13_database"
)

func TestExecuteWithDeadlockRetry_SuccessAfterDeadlock(t *testing.T) {
	attempts := 0
	err := database.ExecuteWithDeadlockRetry(context.Background(), 3, 5*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			return database.ErrDeadlockDetected // 1st attempt deadlocks
		}
		return nil // 2nd attempt succeeds
	})

	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got: %d", attempts)
	}
}

func TestExecuteWithDeadlockRetry_PermanentErrorAborts(t *testing.T) {
	attempts := 0
	permErr := errors.New("foreign key violation")

	err := database.ExecuteWithDeadlockRetry(context.Background(), 3, 5*time.Millisecond, func(ctx context.Context) error {
		attempts++
		return permErr
	})

	if !errors.Is(err, permErr) {
		t.Errorf("expected permanent error returned immediately, got: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected only 1 attempt for permanent error, got: %d", attempts)
	}
}
