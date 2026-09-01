package contextpattern

import (
	"context"
	"sync"
)

// PipelineJob represents work to be processed.
type PipelineJob struct {
	ID    int
	Value int
}

// PipelineResult represents processed outcome.
type PipelineResult struct {
	JobID  int
	Square int
	Err    error
}

// RunCancellationPipeline processes jobs concurrently and terminates cleanly on ctx.Done().
func RunCancellationPipeline(ctx context.Context, jobs []PipelineJob, workerCount int) ([]PipelineResult, error) {
	if workerCount <= 0 {
		workerCount = 4
	}

	jobCh := make(chan PipelineJob, len(jobs))
	resultCh := make(chan PipelineResult, len(jobs))

	// Enqueue jobs
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobCh:
					if !ok {
						return
					}
					// Process job with cancellation check
					select {
					case <-ctx.Done():
						return
					case resultCh <- PipelineResult{
						JobID:  job.ID,
						Square: job.Value * job.Value,
					}:
					}
				}
			}
		}()
	}

	// Close resultCh once all workers finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []PipelineResult
	for res := range resultCh {
		results = append(results, res)
	}

	if err := ctx.Err(); err != nil {
		return results, err
	}

	return results, nil
}
