package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CleanupFunc is a function executed during graceful shutdown.
type CleanupFunc func(ctx context.Context) error

// ShutdownHook holds a named cleanup task and its priority.
type ShutdownHook struct {
	Name     string
	Priority int // Higher priority runs first
	Fn       CleanupFunc
}

// GracefulShutdownManager manages the orderly teardown of application resources.
type GracefulShutdownManager struct {
	mu       sync.Mutex
	hooks    []ShutdownHook
	timeout  time.Duration
	isClosed bool
}

// NewGracefulShutdownManager creates a new shutdown manager with a specified timeout.
func NewGracefulShutdownManager(timeout time.Duration) *GracefulShutdownManager {
	return &GracefulShutdownManager{
		timeout: timeout,
		hooks:   make([]ShutdownHook, 0),
	}
}

// Register adds a cleanup hook. Hooks with higher priority run earlier.
func (m *GracefulShutdownManager) Register(name string, priority int, fn CleanupFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isClosed {
		return
	}

	m.hooks = append(m.hooks, ShutdownHook{
		Name:     name,
		Priority: priority,
		Fn:       fn,
	})
}

// Shutdown executes all registered hooks in order of priority within the configured timeout.
func (m *GracefulShutdownManager) Shutdown(parentCtx context.Context) error {
	m.mu.Lock()
	if m.isClosed {
		m.mu.Unlock()
		return errors.New("shutdown already executed")
	}
	m.isClosed = true

	// Sort hooks by priority descending (simple insertion sort for stable order)
	sorted := make([]ShutdownHook, len(m.hooks))
	copy(sorted, m.hooks)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j].Priority < key.Priority {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(parentCtx, m.timeout)
	defer cancel()

	var errs []error
	for _, hook := range sorted {
		select {
		case <-ctx.Done():
			return fmt.Errorf("shutdown timed out after %v: %w", m.timeout, ctx.Err())
		default:
			if err := hook.Fn(ctx); err != nil {
				errs = append(errs, fmt.Errorf("hook %s failed: %w", hook.Name, err))
			}
		}
	}

	return errors.Join(errs...)
}
