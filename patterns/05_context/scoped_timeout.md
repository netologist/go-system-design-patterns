# Scoped Timeout & Budget Management Pattern

## Overview & Definition
The **Scoped Timeout & Budget Management** pattern calculates and enforces hierarchical, bounded execution deadlines across nested service calls, ensuring that child operations never attempt to allocate a timeout longer than the remaining time budget of their parent request context.

In distributed backend architectures, an incoming HTTP request has an overall SLA deadline (e.g., 2 seconds). When calling a downstream dependency (such as a database query or an external credit check API), creating a child context with `context.WithTimeout(parent, 5*time.Second)` is a bug because the parent will cancel in 2 seconds. The `WithBoundedTimeout` helper clamps the child timeout so it respects $\min(\text{requestedTimeout}, \text{parentRemainingBudget})$.

---

## Problem Statement (Failure scenarios without this pattern)
Naive timeout creation in nested microservice calls leads to severe production issues:
- **Illusory Timeouts**: An HTTP handler with 500ms left before timing out invokes a payment gateway client configured with `Timeout: 3s`. Developers assume the client has 3 seconds to complete, but the request aborts ungracefully at 500ms, making latency budgets misleading.
- **Budget Exhaustion in Loops**: In batch processing workflows or retry loops, executing 3 successive calls with static 1s timeouts inside a 2s parent context causes the 3rd call to fail immediately after starting because the parent budget was already exhausted by prior attempts.
- **Wasted Network Calls**: Launching a heavy downstream call when only 2ms remain of the parent budget guarantees failure before the remote network round-trip can return.

---

## Architectural Mechanism & Flow

```mermaid
flowchart TD
    A[Parent Context with Deadline: e.g. 800ms remaining] --> B["WithBoundedTimeout(parent, maxDuration = 2000ms)"]
    
    B --> C{Parent has Deadline?}
    C -- Yes --> D[Calculate remaining = time.Until(parentDeadline)]
    D --> E{Is remaining < maxDuration?}
    E -- Yes --> F[Clamp Child Timeout to remaining: 800ms]
    E -- No --> G[Use maxDuration: 2000ms]
    C -- No --> G
    
    F --> H[Return Derived Child Context & CancelFunc]
    G --> H
    
    subgraph Budget Inspection
        I[RemainingBudget Function] --> J[Returns exact time.Duration remaining before expiration]
    end
```

### Mathematical Formulation
For any derived child context $C$ created with requested duration $D_{req}$ under parent context $P$ having deadline $T_P$:

$$D_{effective} = \begin{cases} \min(D_{req}, T_P - \text{now()}) & \text{if } P \text{ has deadline} \\ D_{req} & \text{otherwise} \end{cases}$$

---

## Production Best Practices & Pitfalls

### Best Practices
- **Check Remaining Budget Before Network Calls**: If `RemainingBudget(ctx) < 50*time.Millisecond` (insufficient time for network round-trip), abort early without making the network call.
- **Always Call `defer cancel()`**: Every context derived with `WithBoundedTimeout` allocates internal runtime timers; calling the returned `CancelFunc` releases timer resources immediately.
- **Deduct Deadlines for Retries**: In retry loops, re-calculate the remaining context budget on every retry iteration to ensure retries fit within the global SLA.
- **Log Budget Metrics**: Attach remaining deadline budgets to outbound client logs to assist distributed tracing and SLA auditing.

### Common Pitfalls
- **Assuming Child Timeouts Can Extend Deadlines**: Forgetting that in Go, a parent context's cancellation or deadline expiration *always* forces all derived child contexts to cancel, regardless of what timeout was passed to `context.WithTimeout`.
- **Forgetting to Defer the Cancel Function**: Leaking timer goroutines in the Go runtime by omitting `defer cancel()` on high-throughput paths.
- **Hardcoding Unbounded Timeouts**: Using fixed static durations without checking parent context constraints.

---

## Code Walkthrough & Usage

The implementation in `scoped_timeout.go` demonstrates deadline inspection and bounded timeout derivation:

```go
package main

import (
	"context"
	"log"
	"time"

	"patterns/05_context"
)

func CallDownstreamService(parentCtx context.Context) {
	// Request a 2-second timeout, but clamp to whatever is left of parent context
	ctx, cancel := contextpattern.WithBoundedTimeout(parentCtx, 2*time.Second)
	defer cancel()

	log.Printf("Effective child budget: %v (Parent remaining was: %v)",
		contextpattern.RemainingBudget(ctx),
		contextpattern.RemainingBudget(parentCtx))

	// Execute downstream I/O using clamped ctx...
}

func main() {
	// Scenario A: Parent has a generous 5-second deadline
	// Child requested 2s -> Effective child budget is 2s
	parentA, cancelA := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelA()
	log.Println("--- Scenario A (Parent > Requested) ---")
	CallDownstreamService(parentA)

	// Scenario B: Parent has only 400ms left
	// Child requested 2s -> Effective child budget is clamped to ~400ms
	parentB, cancelB := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancelB()
	log.Println("--- Scenario B (Parent < Requested) ---")
	CallDownstreamService(parentB)
}
```
