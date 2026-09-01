package backgroundjobs

import (
	"context"
	"sync"
	"time"
)

// ScheduledTask is a periodic background function.
type ScheduledTask func(ctx context.Context) error

// ScheduledTaskRunner executes a task periodically and supports clean cancellation.
type ScheduledTaskRunner struct {
	interval time.Duration
	task     ScheduledTask
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewScheduledTaskRunner(parentCtx context.Context, interval time.Duration, task ScheduledTask) *ScheduledTaskRunner {
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	ctx, cancel := context.WithCancel(parentCtx)
	return &ScheduledTaskRunner{
		interval: interval,
		task:     task,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start launches the periodic ticker loop.
func (r *ScheduledTaskRunner) Start() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				_ = r.task(r.ctx)
			}
		}
	}()
}

// Stop cleanly cancels the scheduler context and waits for the active execution to complete.
func (r *ScheduledTaskRunner) Stop() {
	r.cancel()
	r.wg.Wait()
}
