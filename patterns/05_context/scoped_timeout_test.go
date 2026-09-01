package contextpattern_test

import (
	"context"
	"testing"
	"time"

	contextpattern "system-design-patterns/patterns/05_context"
)

func TestWithBoundedTimeout_ParentHasShorterDeadline(t *testing.T) {
	// Parent deadline in 50ms
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer parentCancel()

	// Child asks for 500ms
	childCtx, childCancel := contextpattern.WithBoundedTimeout(parentCtx, 500*time.Millisecond)
	defer childCancel()

	childDeadline, _ := childCtx.Deadline()
	parentDeadline, _ := parentCtx.Deadline()

	diff := childDeadline.Sub(parentDeadline)
	if diff > 5*time.Millisecond || diff < -5*time.Millisecond {
		t.Errorf("child deadline should match parent deadline when parent is shorter, diff: %v", diff)
	}
}

func TestWithBoundedTimeout_ChildHasShorterDuration(t *testing.T) {
	// Parent deadline in 5 seconds
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

	// Child asks for 50ms
	childCtx, childCancel := contextpattern.WithBoundedTimeout(parentCtx, 50*time.Millisecond)
	defer childCancel()

	budget := contextpattern.RemainingBudget(childCtx)
	if budget > 60*time.Millisecond {
		t.Errorf("child budget should be around 50ms, got: %v", budget)
	}
}
