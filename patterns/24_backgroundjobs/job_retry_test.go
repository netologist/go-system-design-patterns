package backgroundjobs_test

import (
	"errors"
	"testing"
	"time"

	backgroundjobs "system-design-patterns/patterns/24_backgroundjobs"
)

func TestJobRetryCoordinator_BackoffAndQuarantine(t *testing.T) {
	coordinator := backgroundjobs.NewJobRetryCoordinator(10*time.Millisecond, 100*time.Millisecond)

	job := &backgroundjobs.ManagedJob{
		ID:          "job-100",
		State:       backgroundjobs.StateReady,
		Attempts:    0,
		MaxAttempts: 3,
	}

	testErr := errors.New("network timeout")

	// 1st failure -> RETRYING
	coordinator.HandleJobFailure(job, testErr)
	if job.State != backgroundjobs.StateRetrying || job.Attempts != 1 {
		t.Errorf("expected state RETRYING after attempt 1, got: %+v", job)
	}

	// 2nd failure -> RETRYING with longer delay
	coordinator.HandleJobFailure(job, testErr)
	if job.State != backgroundjobs.StateRetrying || job.Attempts != 2 {
		t.Errorf("expected state RETRYING after attempt 2, got: %+v", job)
	}

	// 3rd failure -> MaxAttempts reached -> QUARANTINED
	coordinator.HandleJobFailure(job, testErr)
	if job.State != backgroundjobs.StateQuarantined || job.Attempts != 3 {
		t.Errorf("expected state QUARANTINED after max attempts, got: %+v", job)
	}
}
