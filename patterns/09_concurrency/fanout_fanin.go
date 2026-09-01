package concurrency

import (
	"context"
	"sync"
)

// FanOut distributes input channel items across worker functions.
func FanOut[T any, R any](ctx context.Context, in <-chan T, workerCount int, workerFn func(ctx context.Context, item T) R) []<-chan R {
	if workerCount <= 0 {
		workerCount = 4
	}

	channels := make([]<-chan R, workerCount)
	for i := range workerCount {
		out := make(chan R)
		channels[i] = out

		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-in:
					if !ok {
						return
					}
					result := workerFn(ctx, item)
					select {
					case <-ctx.Done():
						return
					case out <- result:
					}
				}
			}
		}()
	}

	return channels
}

// FanIn merges multiple input channels into a single unified output channel.
func FanIn[R any](ctx context.Context, channels ...<-chan R) <-chan R {
	merged := make(chan R)
	var wg sync.WaitGroup

	for _, ch := range channels {
		wg.Add(1)
		go func(c <-chan R) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-c:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case merged <- item:
					}
				}
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(merged)
	}()

	return merged
}
