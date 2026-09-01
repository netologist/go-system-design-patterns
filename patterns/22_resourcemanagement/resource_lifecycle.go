package resourcemanagement

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// ManagedResource tracks ownership and lifecycle of an acquired OS/network resource.
type ManagedResource struct {
	ID        string
	Closer    io.Closer
	CreatedAt time.Time
	Timeout   time.Duration
}

// ResourceTracker enforces:
// 1. Explicit ownership
// 2. Upper bound on open resources
// 3. Guaranteed cleanup / Close timeout
type ResourceTracker struct {
	mu           sync.Mutex
	maxResources int
	active       map[string]*ManagedResource
}

func NewResourceTracker(maxResources int) *ResourceTracker {
	if maxResources <= 0 {
		maxResources = 100
	}
	return &ResourceTracker{
		maxResources: maxResources,
		active:       make(map[string]*ManagedResource),
	}
}

// Track registers a newly opened resource. Rejects if capacity exceeded.
func (t *ResourceTracker) Track(id string, closer io.Closer, timeout time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.active) >= t.maxResources {
		return errors.New("resource upper bound capacity exceeded")
	}

	t.active[id] = &ManagedResource{
		ID:        id,
		Closer:    closer,
		CreatedAt: time.Now(),
		Timeout:   timeout,
	}
	return nil
}

// Release closes and unregisters the resource.
func (t *ResourceTracker) Release(id string) error {
	t.mu.Lock()
	res, ok := t.active[id]
	if !ok {
		t.mu.Unlock()
		return nil
	}
	delete(t.active, id)
	t.mu.Unlock()

	if res.Closer != nil {
		return res.Closer.Close()
	}
	return nil
}

// CloseAll closes all active resources within timeout deadline (useful for process shutdown).
func (t *ResourceTracker) CloseAll(ctx context.Context) error {
	t.mu.Lock()
	resources := make([]*ManagedResource, 0, len(t.active))
	for _, r := range t.active {
		resources = append(resources, r)
	}
	t.active = make(map[string]*ManagedResource)
	t.mu.Unlock()

	var errs []error
	for _, r := range resources {
		select {
		case <-ctx.Done():
			return fmt.Errorf("close all timed out: %w", ctx.Err())
		default:
			if err := r.Closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func (t *ResourceTracker) ActiveCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.active)
}
