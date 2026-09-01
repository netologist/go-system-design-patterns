package httpclient_test

import (
	"errors"
	"testing"
	"time"

	httpclient "system-design-patterns/patterns/08_httpclient"
)

func TestCircuitBreaker_FullLifecycle(t *testing.T) {
	cb := httpclient.NewCircuitBreaker(httpclient.CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		CooldownTimeout:  50 * time.Millisecond,
	})

	if cb.State() != httpclient.StateClosed {
		t.Errorf("expected initial state CLOSED, got: %s", cb.State())
	}

	downstreamErr := errors.New("500 internal server error")

	// 1st failure (still closed)
	_ = cb.Execute(func() error { return downstreamErr })
	if cb.State() != httpclient.StateClosed {
		t.Errorf("expected state CLOSED after 1 failure, got: %s", cb.State())
	}

	// 2nd failure -> Trips to OPEN
	_ = cb.Execute(func() error { return downstreamErr })
	if cb.State() != httpclient.StateOpen {
		t.Errorf("expected state OPEN after 2 failures, got: %s", cb.State())
	}

	// Immediate execution should fail fast with ErrCircuitOpen
	executed := false
	err := cb.Execute(func() error {
		executed = true
		return nil
	})
	if !errors.Is(err, httpclient.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
	if executed {
		t.Error("downstream function should not have executed while circuit is OPEN")
	}

	// Wait for cooldown to transition to HALF_OPEN
	time.Sleep(60 * time.Millisecond)

	// 1st success in HALF_OPEN
	_ = cb.Execute(func() error { return nil })
	if cb.State() != httpclient.StateHalfOpen {
		t.Errorf("expected state HALF_OPEN after 1 success, got: %s", cb.State())
	}

	// 2nd success in HALF_OPEN -> Transitions back to CLOSED
	_ = cb.Execute(func() error { return nil })
	if cb.State() != httpclient.StateClosed {
		t.Errorf("expected state CLOSED after 2 successes, got: %s", cb.State())
	}
}
