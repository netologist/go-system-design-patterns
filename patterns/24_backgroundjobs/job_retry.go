package backgroundjobs

import (
	"fmt"
	"time"
)

type JobState string

const (
	StateReady       JobState = "READY"
	StateRetrying    JobState = "RETRYING"
	StateQuarantined JobState = "QUARANTINED"
)

// ManagedJob represents a job tracked by the retry coordinator.
type ManagedJob struct {
	ID           string
	State        JobState
	Attempts     int
	MaxAttempts  int
	NextRunAt    time.Time
	LastError    string
}

// JobRetryCoordinator calculates retry schedules and quarantines poison jobs.
type JobRetryCoordinator struct {
	baseBackoff time.Duration
	maxBackoff  time.Duration
}

func NewJobRetryCoordinator(baseBackoff, maxBackoff time.Duration) *JobRetryCoordinator {
	if baseBackoff <= 0 {
		baseBackoff = 1 * time.Second
	}
	if maxBackoff <= 0 {
		maxBackoff = 1 * time.Hour
	}
	return &JobRetryCoordinator{
		baseBackoff: baseBackoff,
		maxBackoff:  maxBackoff,
	}
}

// HandleJobFailure calculates the next attempt time or quarantines the job if max attempts reached.
func (c *JobRetryCoordinator) HandleJobFailure(job *ManagedJob, failureErr error) {
	job.Attempts++
	job.LastError = failureErr.Error()

	if job.Attempts >= job.MaxAttempts {
		job.State = StateQuarantined
		job.NextRunAt = time.Time{}
		return
	}

	// Exponential backoff: base * 2^(attempts-1)
	multiplier := time.Duration(1 << uint(job.Attempts-1))
	delay := c.baseBackoff * multiplier
	if delay > c.maxBackoff {
		delay = c.maxBackoff
	}

	job.State = StateRetrying
	job.NextRunAt = time.Now().Add(delay)
}

func (j *ManagedJob) String() string {
	return fmt.Sprintf("Job[%s state=%s attempts=%d/%d err=%s]",
		j.ID, j.State, j.Attempts, j.MaxAttempts, j.LastError)
}
