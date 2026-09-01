# Job Retry & Quarantine Pattern (Exponential Backoff with Dead-Letter Handling)

## Overview & Definition
The **Job Retry & Quarantine Pattern** handles transient failures in asynchronous background job processing by calculating exponential backoff retry schedules and isolating permanently failing tasks (poison pills) into a quarantined dead-letter state.

In distributed backend processing, network blips, external API rate limits, and transient database lock contentions regularly cause background job failures. Instead of retrying immediately (which hammers struggling dependencies) or dropping failed tasks permanently, this pattern computes exponential backoff delays ($Delay = Base \times 2^{Attempts-1}$) bounded by a configurable ceiling (`maxBackoff`). When a job exceeds its maximum retry threshold (`MaxAttempts`), it is moved to the `QUARANTINED` state for manual operator inspection and remediation.

---

## Problem Statement
Uncontrolled retries and unhandled poison jobs degrade system performance and disrupt asynchronous workflows.

### Failure Scenarios Without This Pattern
- **Self-Inflicted DDoS on Downstream Services:** Retrying immediately without backoff compounds load on a struggling external API or database, turning a minor hiccup into a prolonged outage.
- **Poison Pill Blocking:** A malformed payload triggers an unrecoverable bug (e.g., divide-by-zero or panic). Retrying the job continuously consumes 100% of worker capacity, starving healthy jobs in the queue.
- **Silent Data Loss:** Dropping a job after a single transient failure loses critical customer data (e.g., invoices, emails, order dispatches).
- **Thundering Herd Retries:** Hundreds of failed jobs retrying simultaneously at fixed intervals create recurring traffic spikes.

---

## Architectural Mechanism & Flow
The `JobRetryCoordinator` evaluates failed job attempts and updates state:

```
[ Worker Executes Job: Fails with Error ]
                    │
                    ▼
 ┌──────────────────────────────────────┐
 │ JobRetryCoordinator.HandleJobFailure │
 └──────────────────┬───────────────────┘
                    │
                    ▼
          [ Increment Attempts ]
          [ Store LastError ]
                    │
          ┌─────────┴─────────┐
          │ Attempts >= Max?  │
          └─────────┬─────────┘
                    │
       ┌────────────┴────────────┐
       ▼                         ▼
     [Yes]                      [No]
       │                         │
       ▼                         ▼
┌─────────────────────────┐  ┌───────────────────────────────────┐
│ State = QUARANTINED     │  │ State = RETRYING                  │
│ NextRunAt = Zero (DLQ)  │  │ Delay = min(Base * 2^(att-1), Max)│
│ Alert SRE / Operator    │  │ NextRunAt = Now + Delay           │
└─────────────────────────┘  └───────────────────────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Enforce Maximum Backoff Ceilings:** Always cap exponential backoff delays (e.g., `maxBackoff = 1 * time.Hour`) so retry delays do not explode to days or weeks.
- **Persist Quarantined Jobs in Dead-Letter Storage:** Store quarantined jobs in dedicated database tables or SQS DLQs with full error stack traces and timestamps.
- **Add Jitter to Retry Delays:** Introduce randomized jitter (e.g., $\pm 20\%$) to exponential backoff delays to prevent synchronized thundering herd retries across workers.
- **Build Operator Replay Tooling:** Provide admin endpoints or CLI tools to inspect, modify, and replay quarantined jobs once the root cause (e.g., bug fix or schema patch) is resolved.

### Pitfalls to Avoid
- **Retrying Non-Retryable Errors:** Retrying business validation errors (e.g., "invalid email address format") is futile. Fail and quarantine permanent errors immediately.
- **Unbounded Max Attempts:** Setting `MaxAttempts = 100` keeps poison pills bouncing around the system for days. Standard production values are typically 3 to 5 attempts.
- **Losing Error History:** Overwriting `LastError` without keeping an audit log makes diagnosing why earlier attempts failed difficult.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `job_retry.go` manages retry calculations and quarantining:

```go
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
	ID          string
	State       JobState
	Attempts    int
	MaxAttempts int
	NextRunAt   time.Time
	LastError   string
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
```

### Production Worker Dispatcher Example

```go
func HandleJobExecution(coordinator *backgroundjobs.JobRetryCoordinator, job *backgroundjobs.ManagedJob) {
    err := executeThirdPartyCall(job.ID)
    if err != nil {
        coordinator.HandleJobFailure(job, err)
        
        if job.State == backgroundjobs.StateQuarantined {
            log.Printf("[ALERT] Job %s quarantined to DLQ! Error: %s", job.ID, job.LastError)
            sendPagerDutyAlert(job)
        } else {
            log.Printf("[RETRY] Job %s scheduled for retry at %v (Attempt %d)", 
                job.ID, job.NextRunAt, job.Attempts)
        }
        return
    }

    log.Printf("[SUCCESS] Job %s completed successfully.", job.ID)
}
```
