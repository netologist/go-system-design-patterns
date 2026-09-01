package backgroundjobs_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	backgroundjobs "system-design-patterns/patterns/24_backgroundjobs"
)

func TestScheduledTaskRunner_PeriodicRunsAndStop(t *testing.T) {
	var runCount atomic.Int32

	task := func(ctx context.Context) error {
		runCount.Add(1)
		return nil
	}

	// 10ms interval
	runner := backgroundjobs.NewScheduledTaskRunner(context.Background(), 10*time.Millisecond, task)
	runner.Start()

	// Allow ~3-4 ticks
	time.Sleep(35 * time.Millisecond)

	runner.Stop()

	runs := runCount.Load()
	if runs < 2 {
		t.Errorf("expected at least 2 periodic task runs, got: %d", runs)
	}
}
