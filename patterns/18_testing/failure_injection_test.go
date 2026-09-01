package testingpattern_test

import (
	"context"
	"errors"
	"testing"
	"time"

	testingpattern "system-design-patterns/patterns/18_testing"
)

func TestFaultInjector_AlwaysFails(t *testing.T) {
	injector := testingpattern.NewFaultInjector(1.0, 0) // 100% failure rate

	executed := false
	err := injector.Execute(context.Background(), func() error {
		executed = true
		return nil
	})

	if !errors.Is(err, testingpattern.ErrInjectedFault) {
		t.Errorf("expected ErrInjectedFault, got: %v", err)
	}
	if executed {
		t.Error("underlying function should not have executed when fault is injected")
	}
}

func TestFaultInjector_ZeroFailureRate(t *testing.T) {
	injector := testingpattern.NewFaultInjector(0.0, 0)

	executed := false
	err := injector.Execute(context.Background(), func() error {
		executed = true
		return nil
	})

	if err != nil || !executed {
		t.Errorf("expected clean execution without fault, got err: %v", err)
	}
}

func TestFaultInjector_DelayCancellation(t *testing.T) {
	// Inject 200ms delay, but cancel context at 20ms
	injector := testingpattern.NewFaultInjector(0.0, 200*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := injector.Execute(ctx, func() error {
		return nil
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded during injected delay, got: %v", err)
	}
}
