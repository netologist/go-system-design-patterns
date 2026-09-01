package backgroundjobs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	backgroundjobs "system-design-patterns/patterns/24_backgroundjobs"
)

func TestVisibilityQueue_VisibilityTimeoutAndAck(t *testing.T) {
	q := backgroundjobs.NewVisibilityQueue(50 * time.Millisecond)
	ctx := context.Background()

	q.Enqueue("job-1", "process_report", 3)

	// 1. Worker A dequeues job
	jobA, err := q.Dequeue(ctx)
	if err != nil || jobA.ID != "job-1" {
		t.Fatalf("worker A failed to dequeue: %v", err)
	}

	// 2. Worker B attempts to dequeue -> None available because job-1 is currently invisible
	_, err = q.Dequeue(ctx)
	if !errors.Is(err, backgroundjobs.ErrNoJobAvailable) {
		t.Errorf("expected ErrNoJobAvailable while job is leased, got: %v", err)
	}

	// 3. Worker A acknowledges job completion
	q.Ack("job-1")

	// 4. After 60ms, job should still not reappear because it was deleted
	time.Sleep(60 * time.Millisecond)
	_, err = q.Dequeue(ctx)
	if !errors.Is(err, backgroundjobs.ErrNoJobAvailable) {
		t.Errorf("expected job deleted after ack")
	}
}

func TestVisibilityQueue_AutoReappearOnWorkerCrash(t *testing.T) {
	q := backgroundjobs.NewVisibilityQueue(20 * time.Millisecond)
	ctx := context.Background()

	q.Enqueue("job-crash", "payload", 3)

	// Worker 1 dequeues and crashes without calling Ack
	j1, _ := q.Dequeue(ctx)
	if j1.Attempts != 1 {
		t.Errorf("expected attempts 1, got: %d", j1.Attempts)
	}

	// Wait for visibility timeout (20ms) to elapse
	time.Sleep(30 * time.Millisecond)

	// Worker 2 dequeues the job that automatically reappeared
	j2, err := q.Dequeue(ctx)
	if err != nil || j2.ID != "job-crash" {
		t.Fatalf("expected job to reappear after worker crash: %v", err)
	}
	if j2.Attempts != 2 {
		t.Errorf("expected attempts incremented to 2, got: %d", j2.Attempts)
	}
}
