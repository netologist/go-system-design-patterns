package concurrency

import (
	"sync"
	"sync/atomic"
)

// GenerateSequence demonstrates Channel Ownership: The producer creates, writes, and closes the channel,
// returning a read-only channel to consumers.
func GenerateSequence(count int) <-chan int {
	out := make(chan int, count)
	go func() {
		defer close(out)
		for i := 1; i <= count; i++ {
			out <- i
		}
	}()
	return out
}

// SafeCounter demonstrates Mutex Discipline (keeping lock scope minimal).
type SafeCounter struct {
	mu    sync.RWMutex
	value int64
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *SafeCounter) Value() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// AtomicCounter demonstrates lock-free concurrent synchronization.
type AtomicCounter struct {
	val atomic.Int64
}

func (a *AtomicCounter) Inc() {
	a.val.Add(1)
}

func (a *AtomicCounter) Value() int64 {
	return a.val.Load()
}
