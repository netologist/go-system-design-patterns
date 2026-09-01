package reliability

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrDependencySaturated = errors.New("dependency bulkhead saturated")

type DependencyWorkerPool struct {
	tokens  chan struct{}
	timeout time.Duration
}

// BulkheadShield wraps and isolates calls to external third-party systems.
type BulkheadShield struct {
	mu    sync.RWMutex
	pools map[string]*DependencyWorkerPool
}

func NewBulkheadShield() *BulkheadShield {
	return &BulkheadShield{
		pools: make(map[string]*DependencyWorkerPool),
	}
}

// RegisterDependency sets up an isolated pool for a named dependency.
func (s *BulkheadShield) RegisterDependency(name string, maxConcurrency int, defaultTimeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokens := make(chan struct{}, maxConcurrency)
	for range maxConcurrency {
		tokens <- struct{}{}
	}

	s.pools[name] = &DependencyWorkerPool{
		tokens:  tokens,
		timeout: defaultTimeout,
	}
}

// Call executes a call against named dependency, isolated from all other dependencies.
func (s *BulkheadShield) Call(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	s.mu.RLock()
	pool, ok := s.pools[name]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unregistered dependency '%s'", name)
	}

	// Non-blocking try acquire
	select {
	case <-pool.tokens:
		defer func() { pool.tokens <- struct{}{} }()

		callCtx, cancel := context.WithTimeout(ctx, pool.timeout)
		defer cancel()

		return fn(callCtx)
	default:
		return fmt.Errorf("%w for '%s'", ErrDependencySaturated, name)
	}
}
