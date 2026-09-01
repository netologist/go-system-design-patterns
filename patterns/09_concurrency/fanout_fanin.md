# Fan-Out / Fan-In Pipeline Pattern

## Overview & Definition

In high-throughput stream processing and data pipelines, single-threaded channel consumption can become a major CPU bottleneck.

The **Fan-Out / Fan-In Pipeline** pattern parallelizes stage processing across multiple goroutines:
* **Fan-Out (`FanOut[T, R]`):** Takes a single input channel `<-chan T` and distributes incoming items across $N$ worker goroutines, each emitting onto its own output channel `<-chan R`.
* **Fan-In (`FanIn[R]`):** Multiplexes multiple worker channels into a single unified output stream `<-chan R`, automatically coordinating channel closure using `sync.WaitGroup`.
* All pipeline stages propagate cancellation and deadlines via `context.Context`.

---

## Problem Statement

Without a structured Fan-Out / Fan-In architecture:

* **Pipeline Head-of-Line Blocking:** In a linear pipeline, if Stage 2 performs an expensive CPU operation (e.g. image hashing or cryptographic signing), the entire stream stalls at the speed of that single stage.
* **Goroutine & Channel Leaks on Early Abort:** Spawning independent goroutines to read from multiple channels without synchronized shutdown leads to dangling goroutines stalled on unread channel writes.
* **Complex Multi-Channel Multiplexing:** Handcrafting ad-hoc `select` loops over dynamic numbers of channels is error-prone and leads to race conditions during channel closure.

---

## Architectural Mechanism & Flow

```
                         Input Stream (<-chan T)
                                    |
                                    v
                     +-----------------------------+
                     |    FanOut(ctx, in, N, fn)   |
                     +-----------------------------+
                                    |
            +-----------------------+-----------------------+
            |                       |                       |
            v                       v                       v
    +---------------+       +---------------+       +---------------+
    | Worker Goroutine 1|   | Worker Goroutine 2|   | Worker Goroutine N|
    | out[0] chan R |       | out[1] chan R |       | out[N-1] chan R |
    +---------------+       +---------------+       +---------------+
            \                       |                       /
             \                      |                      /
              v                     v                     v
                     +-----------------------------+
                     |    FanIn(ctx, outChannels)  |
                     |      (sync.WaitGroup)       |
                     +-----------------------------+
                                    |
                                    v
                        Unified Merged Stream (<-chan R)
```

### Channel Closure Coordination
1. In `FanOut`, each worker owns its output channel and executes `defer close(out)` when the input channel is exhausted or `ctx.Done()` fires.
2. In `FanIn`, a `sync.WaitGroup` tracks each incoming channel drainer. When all input channels close, a dedicated monitoring goroutine executes `close(merged)`.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Keep Channel Ownership Explicit:** The goroutine that writes to a channel must be the sole entity responsible for closing it. Never allow readers or third parties to close channels.
* **Always Select on `ctx.Done()` on Channel Writes:** When sending to output channels (`out <- result`), always include a `case <-ctx.Done(): return` branch. If downstream consumers abort early, worker goroutines will otherwise block indefinitely on channel writes.
* **Avoid Over-Partitioning for Lightweight Work:** If `workerFn` executes in nanoseconds, channel synchronization overhead will exceed computation time. Use batching or single-goroutine pipelines for trivial workloads.

### Common Pitfalls
* **Closing `merged` Channel Multiple Times:** Attempting to close the `merged` channel from inside individual worker loops triggers a panic. Use `sync.WaitGroup` and a single coordinator goroutine.
* **Ignoring Context Cancellation in Workers:** Failing to check `ctx.Done()` in worker loops causes zombie goroutines that outlive HTTP request lifecycles.

---

## Code Walkthrough & Usage

### 1. Implementation (`fanout_fanin.go`)

```go
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
```

### 2. Parallel Processing Example

```go
func ProcessRecords(ctx context.Context, records []Record) []EnrichedRecord {
    // 1. Source Generator
    in := make(chan Record, len(records))
    for _, r := range records {
        in <- r
    }
    close(in)

    // 2. Fan-Out across 8 parallel enrichers
    workerChannels := FanOut(ctx, in, 8, func(c context.Context, r Record) EnrichedRecord {
        return enrichWithMetadata(c, r)
    })

    // 3. Fan-In to single stream
    out := FanIn(ctx, workerChannels...)

    // 4. Collect results
    var enriched []EnrichedRecord
    for res := range out {
        enriched = append(enriched, res)
    }

    return enriched
}
```
