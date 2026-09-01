# Resource Lifecycle Management Pattern

## Overview & Definition
The **Resource Lifecycle Management Pattern** provides explicit, deterministic tracking, bounding, and cleanup of operating system and network resources (such as open file handles, temporary disk files, child processes, OS sockets, and raw connections).

In Go, while memory is garbage collected automatically, external OS resources are not reclaimed until their underlying file descriptors or network handles are explicitly closed via `io.Closer`. If errors, early returns, panics, or uncoordinated shutdowns bypass standard `defer closer.Close()` statements, the process slowly leaks file descriptors, eventually triggering fatal `EMFILE: too many open files` kernel errors.

The `ResourceTracker` enforces three key guarantees:
1. **Explicit Resource Ownership & Tracking:** Centralized registry of all open active resources.
2. **Hard Upper Bound on Open Resources:** Rejects new allocations when capacity is exceeded to prevent system exhaustion.
3. **Deterministic Bulk Cleanup:** Gracefully closes all active resources within a context deadline during graceful shutdown.

---

## Problem Statement
Relying solely on implicit or uncoordinated cleanup of OS resources leads to resource exhaustion in long-running services.

### Failure Scenarios Without This Pattern
- **File Descriptor Leaks (`EMFILE` Crashes):** Sockets or file descriptors left open during transient network errors accumulate over days, until the process cannot open any new connections or accept HTTP requests.
- **Unbounded Resource Allocation:** An unexpected batch processing spike attempts to open 50,000 files simultaneously, exhausting system memory and file table limits.
- **Dangling Resources Across Shutdowns:** When the application receives `SIGTERM`, active temporary files and database transactions are left in an uncommitted, locked, or orphaned state on disk.
- **Silent Close Failures:** Closing errors are ignored in naive defer calls (`defer f.Close()`), hiding disk I/O errors or network sync failures.

---

## Architectural Mechanism & Flow
The `ResourceTracker` coordinates resource registration, active limits, individual releases, and process-wide bulk shutdown:

```
[ Application Opens Resource (e.g., File, Socket) ]
                       │
                       ▼
        ┌─────────────────────────────┐
        │  ResourceTracker.Track(id)  │
        └──────────────┬──────────────┘
                       │
         ┌─────────────┴─────────────┐
         ▼                           ▼
  [ len >= maxResources ]      [ len < maxResources ]
         │                           │
         ▼                           ▼
   [ Reject with Error ]       [ Register ManagedResource ]
                               [ Store ID, Closer, Time ]
                                     │
                 ┌───────────────────┴───────────────────┐
                 ▼                                       ▼
        [ Normal Operation ]                    [ Shutdown Event ]
                 │                                       │
                 ▼                                       ▼
    ┌───────────────────────────┐           ┌───────────────────────────┐
    │ ResourceTracker.Release() │           │ ResourceTracker.CloseAll()│
    │ - Delete from registry    │           │ - Extract all active      │
    │ - Execute closer.Close()  │           │ - Loop & Close() with Ctx │
    └───────────────────────────┘           │ - Aggregate errors.Join() │
                                            └───────────────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Bounded Resource Ceilings:** Set conservative maximum thresholds for concurrent open resources well below the OS `ulimit -n` limits.
- **Error Aggregation with `errors.Join`:** In Go 1.20+, use `errors.Join` to accumulate close errors across multiple resources without discarding individual failure details.
- **Context-Aware Bulk Shutdown:** Pass `context.Context` to `CloseAll(ctx)` so that hung or unresponsive external resources do not block process termination.
- **Thread-Safe Registry:** Protect registry maps using mutexes (`sync.Mutex`), copying references out of the map before releasing the lock during bulk closing to prevent deadlock.

### Pitfalls to Avoid
- **Holding Mutex Locks Across Slow `Close()` Calls:** If a network socket's `Close()` blocks on a TCP linger timeout while holding the registry lock, all other operations will stall. Always release the lock before calling `Close()`.
- **Ignoring Return Values in Defer:** `defer f.Close()` silently ignores error returns. In write pipelines, write flushes often only fail upon `Close()`.
- **Duplicate Key Collisions:** Ensure resource IDs are unique (e.g., UUID or structured hierarchical keys) so new registrations do not overwrite active resources.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `resource_lifecycle.go` manages resource lifecycles:

```go
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
```

### Production File Processing Example

```go
func ProcessBatchFiles(ctx context.Context, tracker *resourcemanagement.ResourceTracker, filePaths []string) error {
    for _, path := range filePaths {
        f, err := os.Open(path)
        if err != nil {
            return err
        }

        // Track resource with bounded ceiling
        if err := tracker.Track(path, f, 5*time.Second); err != nil {
            _ = f.Close()
            return fmt.Errorf("failed to allocate resource for %s: %w", path, err)
        }

        // Process file...
        _ = processFile(f)

        // Deterministically release resource
        _ = tracker.Release(path)
    }
    return nil
}
```
