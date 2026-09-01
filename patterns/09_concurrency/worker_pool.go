package concurrency

import (
	"context"
	"sync"
)

// Task represents an executable unit of work.
type Task[T any, R any] struct {
	Input T
	Fn    func(ctx context.Context, input T) (R, error)
}

// TaskResult holds the outcome of a Task.
type TaskResult[R any] struct {
	Output R
	Err    error
}

// BoundedWorkerPool processes tasks using a fixed number of worker goroutines.
type BoundedWorkerPool[T any, R any] struct {
	workerCount int
	taskQueue   chan Task[T, R]
	results     chan TaskResult[R]
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewBoundedWorkerPool initializes the pool and starts workers.
func NewBoundedWorkerPool[T any, R any](parentCtx context.Context, workerCount int, queueCapacity int) *BoundedWorkerPool[T, R] {
	if workerCount <= 0 {
		workerCount = 4
	}
	if queueCapacity <= 0 {
		queueCapacity = 100
	}

	ctx, cancel := context.WithCancel(parentCtx)

	pool := &BoundedWorkerPool[T, R]{
		workerCount: workerCount,
		taskQueue:   make(chan Task[T, R], queueCapacity),
		results:     make(chan TaskResult[R], queueCapacity),
		ctx:         ctx,
		cancel:      cancel,
	}

	pool.start()
	return pool
}

func (p *BoundedWorkerPool[T, R]) start() {
	for range p.workerCount {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-p.ctx.Done():
					return
				case task, ok := <-p.taskQueue:
					if !ok {
						return
					}
					out, err := task.Fn(p.ctx, task.Input)
					select {
					case <-p.ctx.Done():
						return
					case p.results <- TaskResult[R]{Output: out, Err: err}:
					}
				}
			}
		}()
	}
}

// Submit enqueues a task. Blocks if queue is full or returns false if context canceled.
func (p *BoundedWorkerPool[T, R]) Submit(task Task[T, R]) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.taskQueue <- task:
		return true
	}
}

// ResultsChannel returns the read-only output channel.
func (p *BoundedWorkerPool[T, R]) Results() <-chan TaskResult[R] {
	return p.results
}

// Shutdown closes the task queue, waits for workers to finish, and closes results.
func (p *BoundedWorkerPool[T, R]) Shutdown() {
	close(p.taskQueue)
	p.wg.Wait()
	close(p.results)
	p.cancel()
}
