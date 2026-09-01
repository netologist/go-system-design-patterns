# Error Group with Context Cancellation

## Overview & Definition

When orchestrating multiple concurrent sub-tasks (e.g., querying 4 microservices in parallel to render a dashboard), managing individual `sync.WaitGroup` counters, shared error mutexes, and context cancellations by hand is error-prone.

The **Error Group** pattern (analogous to `golang.org/x/sync/errgroup`) provides a synchronized abstraction (`Group`) that:
1. Spawns tasks concurrently via `g.Go(fn func() error)`.
2. Automatically propagates a derived `context.Context` (`WithContext(ctx)`) to all sibling tasks.
3. Cancels the shared context on the **very first error** returned by any task, immediately terminating all remaining sibling operations and avoiding wasted computation.
4. Collects and returns the first non-nil error when `g.Wait()` unblocks.

---

## Problem Statement

Manual coordination of concurrent tasks without an Error Group leads to common failure modes:

* **Wasted Resources on Early Failures (Zombie Tasks):** If Task 1 of 5 fails immediately with a fatal database error, standard `sync.WaitGroup` waits for the remaining 4 slow network tasks to complete, wasting CPU, memory, and database connections.
* **Complex Multi-Error Aggregation & Race Conditions:** Storing errors from multiple goroutines into a shared slice or variable without strict mutex synchronization causes data races detected by Go's `-race` detector.
* **Deadlocks on Uncaught Errors:** If an error condition causes a goroutine to return early without calling `wg.Done()`, `wg.Wait()` blocks forever, freezing the application.

---

## Architectural Mechanism & Flow

```
                      WithContext(parentCtx) -> (*Group, ctx)
                                    |
            +-----------------------+-----------------------+
            |                       |                       |
            v                       v                       v
       g.Go(Task 1)            g.Go(Task 2)            g.Go(Task 3)
            |                       |                       |
       [Executes]              [Executes]              [Executes]
            |                       |                       |
      [Returns Error!]              |                       |
            |                       |                       |
    [Lock errMu]                    |                       |
    [Record first err]              |                       |
    [cancel(err)]                   |                       |
            |                       |                       |
            +-------------+---------+                       |
                          |                                 |
                          v                                 v
                 [ctx.Done() triggers]             [ctx.Done() triggers]
                 [Task 2 aborts early]             [Task 3 aborts early]
                          |                                 |
                          +----------------+----------------+
                                           |
                                           v
                                    g.Wait() returns
                                    First Recorded Error
```

### Context Cancellation via `context.WithCancelCause`
In modern Go (1.20+), `context.WithCancelCause` attaches the triggering error directly to the context cancellation, allowing downstream code to inspect `context.Cause(ctx)`.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Pass the Derived Context to Every Child Operation:** Ensure database calls, HTTP requests, and downstream RPCs within `g.Go` use the `ctx` returned by `WithContext(parentCtx)`, not `parentCtx` or `context.Background()`.
* **Cap Concurrency When Necessary:** If launching thousands of dynamic subtasks, pair `Group` with a bounded worker pool or semaphore to avoid allocating thousands of concurrent goroutines.
* **Return Specific Sentinel Errors:** Return typed errors from sub-tasks so callers can differentiate between network timeouts, authorization failures, and validation errors.

### Common Pitfalls
* **Variable Shadowing in Loops:** In Go versions prior to 1.22, capturing loop iteration variables directly inside `g.Go(func() error { ... })` caused race conditions. While Go 1.22 fixes loop variable scoping, explicitly passing parameters to closures is still considered clean practice.
* **Ignoring Context Inside Goroutine:** If the function inside `g.Go` ignores `ctx.Done()`, cancelling the group will not interrupt the running work.

---

## Code Walkthrough & Usage

### 1. Implementation (`errgroup.go`)

```go
package concurrency

import (
	"context"
	"sync"
)

// Group coordinates a collection of goroutines, canceling their shared context upon the first error.
type Group struct {
	cancel func(error)
	wg     sync.WaitGroup
	errMu  sync.Mutex
	err    error
}

// WithContext returns a new Group and associated Context derived from ctx.
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Group{cancel: cancel}, ctx
}

// Go calls the given function in a new goroutine.
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errMu.Lock()
			if g.err == nil {
				g.err = err
				if g.cancel != nil {
					g.cancel(err)
				}
			}
			g.errMu.Unlock()
		}
	}()
}

// Wait blocks until all goroutines finish, returning the first non-nil error.
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(nil)
	}
	return g.err
}
```

### 2. Microservice Aggregation Example

```go
type UserDashboard struct {
    Profile   UserProfile
    Orders    []Order
    Analytics UserAnalytics
}

func FetchDashboard(parentCtx context.Context, userID string) (*UserDashboard, error) {
    g, ctx := WithContext(parentCtx)
    var dash UserDashboard

    // Sub-task 1: Fetch Profile
    g.Go(func() error {
        profile, err := userClient.GetProfile(ctx, userID)
        if err != nil {
            return fmt.Errorf("profile fetch failed: %w", err)
        }
        dash.Profile = profile
        return nil
    })

    // Sub-task 2: Fetch Orders
    g.Go(func() error {
        orders, err := orderClient.GetRecentOrders(ctx, userID)
        if err != nil {
            return fmt.Errorf("orders fetch failed: %w", err)
        }
        dash.Orders = orders
        return nil
    })

    // Sub-task 3: Fetch Analytics
    g.Go(func() error {
        analytics, err := analyticsClient.GetStats(ctx, userID)
        if err != nil {
            return fmt.Errorf("analytics fetch failed: %w", err)
        }
        dash.Analytics = analytics
        return nil
    })

    // Wait for all to succeed, or abort immediately on first error
    if err := g.Wait(); err != nil {
        return nil, err
    }

    return &dash, nil
}
```
