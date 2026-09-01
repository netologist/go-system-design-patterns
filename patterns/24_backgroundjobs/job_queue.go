package backgroundjobs

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNoJobAvailable = errors.New("no jobs available in queue")
)

// Job represents a background task in the queue.
type Job struct {
	ID           string
	Payload      string
	VisibleAfter time.Time
	Attempts     int
	MaxAttempts  int
}

// VisibilityQueue manages background jobs with visibility timeout (SQS/RabbitMQ-style).
type VisibilityQueue struct {
	mu                sync.Mutex
	jobs              map[string]*Job
	visibilityTimeout time.Duration
}

func NewVisibilityQueue(visibilityTimeout time.Duration) *VisibilityQueue {
	if visibilityTimeout <= 0 {
		visibilityTimeout = 30 * time.Second
	}
	return &VisibilityQueue{
		jobs:              make(map[string]*Job),
		visibilityTimeout: visibilityTimeout,
	}
}

// Enqueue adds a job to the queue.
func (q *VisibilityQueue) Enqueue(id, payload string, maxAttempts int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.jobs[id] = &Job{
		ID:           id,
		Payload:      payload,
		VisibleAfter: time.Now(),
		Attempts:     0,
		MaxAttempts:  maxAttempts,
	}
}

// Dequeue fetches the next visible job and hides it for visibilityTimeout duration.
func (q *VisibilityQueue) Dequeue(ctx context.Context) (*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for _, j := range q.jobs {
		if now.After(j.VisibleAfter) {
			j.Attempts++
			j.VisibleAfter = now.Add(q.visibilityTimeout)
			return &Job{
				ID:           j.ID,
				Payload:      j.Payload,
				Attempts:     j.Attempts,
				MaxAttempts:  j.MaxAttempts,
				VisibleAfter: j.VisibleAfter,
			}, nil
		}
	}

	return nil, ErrNoJobAvailable
}

// Heartbeat extends the visibility timeout of a currently running job.
func (q *VisibilityQueue) Heartbeat(id string, extension time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, ok := q.jobs[id]
	if !ok {
		return errors.New("job not found")
	}

	j.VisibleAfter = time.Now().Add(extension)
	return nil
}

// Ack acknowledges completion and permanently removes the job.
func (q *VisibilityQueue) Ack(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.jobs, id)
}
