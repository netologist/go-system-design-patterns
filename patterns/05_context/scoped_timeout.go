package contextpattern

import (
	"context"
	"time"
)

// WithBoundedTimeout derives a context with a timeout that never exceeds the parent's remaining deadline.
func WithBoundedTimeout(parent context.Context, maxDuration time.Duration) (context.Context, context.CancelFunc) {
	if parentDeadline, ok := parent.Deadline(); ok {
		remaining := time.Until(parentDeadline)
		if remaining < maxDuration {
			// Parent will expire sooner than maxDuration; adopt remaining parent budget
			return context.WithTimeout(parent, remaining)
		}
	}
	return context.WithTimeout(parent, maxDuration)
}

// RemainingBudget calculates remaining time before context deadline (or zero if expired / no deadline).
func RemainingBudget(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 0 {
			return 0
		}
		return remaining
	}
	return 0
}
