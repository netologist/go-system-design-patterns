package contextpattern_test

import (
	"context"
	"errors"
	"testing"

	contextpattern "system-design-patterns/patterns/05_context"
)

func TestRunCancellationPipeline_Success(t *testing.T) {
	jobs := make([]contextpattern.PipelineJob, 50)
	for i := range 50 {
		jobs[i] = contextpattern.PipelineJob{ID: i, Value: i}
	}

	results, err := contextpattern.RunCancellationPipeline(context.Background(), jobs, 4)
	if err != nil {
		t.Fatalf("expected pipeline to succeed, got: %v", err)
	}

	if len(results) != 50 {
		t.Errorf("expected 50 results, got: %d", len(results))
	}
}

func TestRunCancellationPipeline_CancelMidway(t *testing.T) {
	jobs := make([]contextpattern.PipelineJob, 500)
	for i := range 500 {
		jobs[i] = contextpattern.PipelineJob{ID: i, Value: i}
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately after starting
	cancel()

	results, err := contextpattern.RunCancellationPipeline(ctx, jobs, 2)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}

	if len(results) == 500 {
		t.Errorf("pipeline did not cancel early, processed all 500 items")
	}
}
