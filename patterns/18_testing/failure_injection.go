package testingpattern

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"
)

var ErrInjectedFault = errors.New("injected simulated dependency fault")

// FaultInjector allows simulating upstream/downstream network failures, timeouts, and latency spikes.
type FaultInjector struct {
	FailureRate float64       // 0.0 = no failure, 1.0 = always fail
	ExtraDelay  time.Duration // Artificial latency
	CustomError error
}

func NewFaultInjector(failureRate float64, extraDelay time.Duration) *FaultInjector {
	return &FaultInjector{
		FailureRate: failureRate,
		ExtraDelay:  extraDelay,
		CustomError: ErrInjectedFault,
	}
}

// Execute wraps an operation, conditionally injecting delay and failures based on configured rules.
func (f *FaultInjector) Execute(ctx context.Context, fn func() error) error {
	// Inject latency
	if f.ExtraDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.ExtraDelay):
		}
	}

	// Inject error based on probability
	if f.FailureRate > 0 && rand.Float64() < f.FailureRate {
		if f.CustomError != nil {
			return f.CustomError
		}
		return ErrInjectedFault
	}

	return fn()
}
