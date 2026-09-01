package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	lifecycle "system-design-patterns/patterns/01_lifecycle"
)

type mockPinger struct {
	delay time.Duration
	err   error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.err
}

func TestStartupValidator_Success(t *testing.T) {
	v := lifecycle.NewStartupValidator(1 * time.Second)
	v.AddDependency("PostgreSQL", true, 200*time.Millisecond, &mockPinger{})
	v.AddDependency("Redis", true, 200*time.Millisecond, &mockPinger{})
	v.AddDependency("Kafka", false, 200*time.Millisecond, &mockPinger{})

	err := v.ValidateAll(context.Background())
	if err != nil {
		t.Fatalf("expected all checks to succeed, got: %v", err)
	}
}

func TestStartupValidator_CriticalFailure(t *testing.T) {
	v := lifecycle.NewStartupValidator(1 * time.Second)
	v.AddDependency("PostgreSQL", true, 200*time.Millisecond, &mockPinger{err: errors.New("connection refused")})
	v.AddDependency("Redis", false, 200*time.Millisecond, &mockPinger{})

	err := v.ValidateAll(context.Background())
	if err == nil {
		t.Fatal("expected critical dependency failure error, got nil")
	}
}

func TestStartupValidator_NonCriticalFailureIgnored(t *testing.T) {
	v := lifecycle.NewStartupValidator(1 * time.Second)
	v.AddDependency("PostgreSQL", true, 200*time.Millisecond, &mockPinger{})
	v.AddDependency("OptionalAnalytics", false, 200*time.Millisecond, &mockPinger{err: errors.New("analytics unreachable")})

	err := v.ValidateAll(context.Background())
	if err != nil {
		t.Fatalf("expected non-critical failure to allow startup, got: %v", err)
	}
}

func TestStartupValidator_Timeout(t *testing.T) {
	v := lifecycle.NewStartupValidator(50 * time.Millisecond)
	v.AddDependency("SlowDB", true, 200*time.Millisecond, &mockPinger{delay: 300 * time.Millisecond})

	err := v.ValidateAll(context.Background())
	if err == nil {
		t.Fatal("expected timeout error for slow dependency, got nil")
	}
}
