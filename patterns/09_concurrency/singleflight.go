package concurrency

import (
	"sync"
)

type call[T any] struct {
	wg     sync.WaitGroup
	val    T
	err    error
	shared bool
}

// SingleflightGroup executes only one in-flight execution for a given key.
type SingleflightGroup[T any] struct {
	mu sync.Mutex
	m  map[string]*call[T]
}

func NewSingleflightGroup[T any]() *SingleflightGroup[T] {
	return &SingleflightGroup[T]{
		m: make(map[string]*call[T]),
	}
}

// Do executes fn and returns the result, ensuring that only one execution is in-flight for a given key at a time.
// If a duplicate comes in, the duplicate caller waits for the original to complete and receives the same results.
func (g *SingleflightGroup[T]) Do(key string, fn func() (T, error)) (v T, err error, shared bool) {
	g.mu.Lock()
	if c, ok := g.m[key]; ok {
		c.shared = true
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}

	c := new(call[T])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err, c.shared
}
