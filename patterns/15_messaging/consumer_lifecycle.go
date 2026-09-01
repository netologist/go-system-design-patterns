package messaging

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ConsumerTask represents a single message handler invocation.
type ConsumerTask func(ctx context.Context) error

// GracefulConsumer manages concurrent worker routines processing messages with graceful drain shutdown.
type GracefulConsumer struct {
	concurrency int
	msgQueue    chan ConsumerTask
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	closed      bool
	mu          sync.Mutex
}

func NewGracefulConsumer(parentCtx context.Context, concurrency int, queueBuffer int) *GracefulConsumer {
	if concurrency <= 0 {
		concurrency = 4
	}
	if queueBuffer <= 0 {
		queueBuffer = 50
	}

	ctx, cancel := context.WithCancel(parentCtx)

	c := &GracefulConsumer{
		concurrency: concurrency,
		msgQueue:    make(chan ConsumerTask, queueBuffer),
		ctx:         ctx,
		cancel:      cancel,
	}

	c.start()
	return c
}

func (c *GracefulConsumer) start() {
	for range c.concurrency {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			for task := range c.msgQueue {
				_ = task(c.ctx)
			}
		}()
	}
}

// Enqueue submits a message handler to the consumer pool.
func (c *GracefulConsumer) Enqueue(task ConsumerTask) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("consumer is shutting down")
	}

	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	case c.msgQueue <- task:
		return nil
	}
}

// Shutdown stops accepting new messages, waits for in-flight tasks to complete, or times out.
func (c *GracefulConsumer) Shutdown(timeout time.Duration) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	close(c.msgQueue)
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.cancel()
		return nil
	case <-time.After(timeout):
		c.cancel() // Force cancel remaining tasks
		return errors.New("consumer shutdown timed out waiting for in-flight messages")
	}
}
