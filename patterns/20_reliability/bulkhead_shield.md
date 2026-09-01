# Bulkhead Shield Pattern (Dependency Concurrency Isolation)

## 1. Overview & Concept
The **Bulkhead Shield Pattern** isolates resources (goroutines, connection pools, and execution slots) allocated to individual downstream dependencies, preventing a slow or failing external service from consuming all available server resources and bringing down the entire host application. Named after the watertight bulkheads in ship hulls that compartmentalize leaks to prevent total sinking, this pattern creates strict concurrency walls around each distinct external integration (e.g., Payment Gateway, SMS Provider, Email API, Fraud Engine).

If one external dependency slows down or becomes unresponsive, only its dedicated bulkhead pool is saturated; all other unrelated features (such as user authentication, catalog browsing, and cart management) continue executing at full performance.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
In distributed microservice backends, unisolated external network calls are the most common source of cascading failures.

### Failure Scenarios Without This Pattern
- **Goroutine & Thread Starvation:** A third-party KYC verification service experiences latency spikes from 100ms to 15 seconds. Incoming KYC requests exhaust the Go runtime's worker capacity, blocking core user login and checkout endpoints.
- **Cascading Connection Pool Depletion:** Slow database queries or API calls hold shared connection pool sockets open, starving time-sensitive transactions.
- **Single Point of Dependency Failure:** A non-critical notification provider (e.g., marketing push notifications) hangs, locking HTTP handler threads until the whole application server crashes.
- **Resource Domino Effect:** Timeouts on one service cause retry storms from upstream clients, spreading failure through the entire microservice graph.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `BulkheadShield` maintains isolated token-based worker pools (`chan struct{}`) per named dependency. Requests attempt non-blocking acquisition of a token before executing the network function:

```
  [ Incoming Application Calls to External Dependencies ]
                            │
        ┌───────────────────┴───────────────────┐
        ▼                                       ▼
 [ Call("payment_gateway") ]             [ Call("sms_provider") ]
        │                                       │
        ▼                                       ▼
 ┌──────────────────────┐                ┌──────────────────────┐
 │ Bulkhead: Payment    │                │ Bulkhead: SMS        │
 │ Capacity: 20 tokens  │                │ Capacity: 5 tokens   │
 └──────────┬───────────┘                └──────────┬───────────┘
            │                                       │
     Acquire Token?                          Acquire Token?
      ┌─────┴─────┐                           ┌─────┴─────┐
      │           │                           │           │
    [Yes]        [No]                       [Yes]        [No]
      │           │                           │           │
      ▼           ▼                           ▼           ▼
 [ Execute ] [Return 503 /               [ Execute ] [Return 503 /
   Payment     ErrDependencySaturated]     SMS         ErrDependencySaturated]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Combine with Timeouts:** Always enforce a tight `context.WithTimeout` on calls within the bulkhead so individual slow calls release tokens promptly.
- **Non-Blocking Token Acquisition (`select-default`):** Use a Go buffered channel with non-blocking `select` (`case <-tokens: ... default: return ErrDependencySaturated`) so caller goroutines fail immediately without queueing.
- **Token Return in Defer:** Always release the token back to the channel in a `defer` statement to prevent token leaks on panics or unexpected errors.
- **Per-Dependency Telemetry:** Track active tokens and saturation rejections per dependency in Prometheus (`bulkhead_active_concurrency{dependency="..."}`, `bulkhead_rejected_total{dependency="..."}`).

### Pitfalls to Avoid
- **Shared Global Limits Across Dependencies:** Grouping multiple unrelated third-party providers under a single pool defeats the purpose of isolation.
- **Excessive Queueing Within Bulkhead:** Storing thousands of blocked requests in unbounded queues introduces severe latency without increasing downstream throughput. Fail fast instead.
- **Ignoring Dependency Criticality:** Set higher concurrency limits for high-throughput, low-latency critical dependencies (e.g., Redis, primary DB) and tight limits on slow third-party REST APIs.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/20_reliability/bulkhead_shield.go`
```go
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
```

### Production Service Integration Example
```go
type PaymentService struct {
    shield *reliability.BulkheadShield
    client *http.Client
}

func NewPaymentService() *PaymentService {
    shield := reliability.NewBulkheadShield()
    // Allocate max 20 concurrent payment calls, 3s timeout
    shield.RegisterDependency("stripe", 20, 3*time.Second)
    // Allocate max 5 concurrent SMS notifications, 1s timeout
    shield.RegisterDependency("twilio", 5, 1*time.Second)

    return &PaymentService{
        shield: shield,
        client: &http.Client{Timeout: 5 * time.Second},
    }
}

func (s *PaymentService) ChargeCustomer(ctx context.Context, amount int64) error {
    return s.shield.Call(ctx, "stripe", func(callCtx context.Context) error {
        // Execute HTTP request to Stripe using callCtx bounded timeout
        return s.executeStripeCharge(callCtx, amount)
    })
}
```
