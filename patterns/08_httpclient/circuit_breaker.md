# Circuit Breaker Pattern

## Overview & Definition

When a downstream dependency (database, microservice, third-party API) fails or experiences severe latency, continuously sending requests wastes thread capacity and prevents the remote service from recovering.

The **Circuit Breaker** pattern acts as an electrical circuit breaker for network calls:
* **CLOSED (Normal Operation):** All requests pass through to the downstream service. The circuit breaker counts consecutive failures.
* **OPEN (Tripped / Failing Fast):** If consecutive failures reach `FailureThreshold`, the circuit trips to `OPEN`. All subsequent calls fail immediately with `ErrCircuitOpen` without hitting the network, saving CPU and protecting the downstream service.
* **HALF_OPEN (Trial Recovery):** After a `CooldownTimeout`, the circuit enters `HALF_OPEN` to permit a limited number of trial probe requests. If `SuccessThreshold` consecutive trial calls succeed, the circuit resets to `CLOSED`. If any trial call fails, it trips back to `OPEN` immediately.

---

## Problem Statement

Without a circuit breaker, systems suffer from cascading failures across distributed architectures:

* **Resource Starvation via Latency Queuing:** When a downstream service becomes unresponsive, calling goroutines block waiting for HTTP timeouts. Under 500 RPS, within seconds thousands of goroutines stall, consuming thread stacks, heap memory, and connection pools until the upstream caller crashes.
* **Denying Downstream Recovery:** An overloaded database or service attempting to recover after a restart is repeatedly hammered by backlogged client traffic, prolonging outages.
* **Poor User Experience:** Users experience multi-second spinning delays before receiving 504 Gateway Timeouts, rather than immediate fallback responses or cached values.

---

## Architectural Mechanism & Flow

```
                      State Transition Diagram

              +------------------------------------------------+
              |                                                |
              |     +-------------------+                      |
              |     |                   | (Failures >= Threshold)
              |     |      CLOSED       |-------------------+  |
              |     | (Normal Traffic)  |                   |  |
              |     +-------------------+                   v  v
              |               ^                     +---------------+
              |               |                     |               |
              | (Successes >= |                     |     OPEN      |
              |    Threshold) |                     |  (Fail Fast)  |
              |               |                     |               |
              |     +-------------------+           +---------------+
              |     |                   |                   |
              +-----|     HALF_OPEN     |<------------------+
                    |  (Probe Traffic)  |   (After CooldownTimeout)
                    +-------------------+
```

### Execution Flow
```
                      Caller calls cb.Execute(fn)
                                  |
                                  v
                        [Acquire Mutex Lock]
                                  |
                                  v
                        [Check Cooldown Expiry]
                     (OPEN & time > Cooldown -> HALF_OPEN)
                                  |
                                  v
                         [Evaluate State]
                           /           \
                     [Is OPEN]      [Is CLOSED / HALF_OPEN]
                        /                  \
                       v                    v
              [Release Lock]          [Release Lock]
              [Return ErrCircuitOpen] [Execute fn()]
                                            |
                                  +---------+---------+
                                  |                   |
                                  v                   v
                             [fn() returns err]  [fn() returns nil]
                                  |                   |
                           [Acquire Lock]      [Acquire Lock]
                           [recordFailure]     [recordSuccess]
                           [Release Lock]      [Release Lock]
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Provide Fallback Mechanisms:** When `cb.Execute()` returns `ErrCircuitOpen`, return a cached snapshot, a degraded response, or a polite queue notification rather than an unformatted error.
* **Scope Breakers Granularly:** Instantiate separate `CircuitBreaker` instances per dependency, domain, or route (e.g. `cbPayments`, `cbNotifications`, `cbInventory`) rather than one global instance.
* **Instrument State Transitions:** Emit metrics on state changes (`circuit_breaker_state{name="payments", state="OPEN"}`) and alert when a breaker trips.

### Common Pitfalls
* **Counting Client Errors (4xx) as Circuit Failures:** If a client sends invalid query parameters (HTTP 400 or 404), do not count these as downstream system failures. Only record 5xx server errors, network timeouts, and TCP connection refused errors.
* **Holding Locks During Network Calls:** Never hold the circuit breaker's mutex lock while executing `fn()`. Locks should only be held briefly while checking or updating internal state counters.

---

## Code Walkthrough & Usage

### 1. Implementation (`circuit_breaker.go`)

```go
package httpclient

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is OPEN: downstream service failing")

type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"
	StateOpen     CircuitState = "OPEN"
	StateHalfOpen CircuitState = "HALF_OPEN"
)

type CircuitBreakerConfig struct {
	FailureThreshold int           // Consecutive failures before opening
	SuccessThreshold int           // Consecutive successes in HALF_OPEN before closing
	CooldownTimeout  time.Duration // Time to wait in OPEN before moving to HALF_OPEN
}

type CircuitBreaker struct {
	mu                  sync.Mutex
	config              CircuitBreakerConfig
	state               CircuitState
	consecutiveFails    int
	consecutiveSuccess int
	lastStateChange     time.Time
}

func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.CooldownTimeout <= 0 {
		cfg.CooldownTimeout = 10 * time.Second
	}

	return &CircuitBreaker{
		config:          cfg,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkStateTransitionLocked()
	return cb.state
}

func (cb *CircuitBreaker) checkStateTransitionLocked() {
	if cb.state == StateOpen && time.Since(cb.lastStateChange) >= cb.config.CooldownTimeout {
		cb.state = StateHalfOpen
		cb.consecutiveSuccess = 0
		cb.consecutiveFails = 0
		cb.lastStateChange = time.Now()
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	cb.checkStateTransitionLocked()

	if cb.state == StateOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}
	cb.mu.Unlock()

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailureLocked()
		return err
	}

	cb.recordSuccessLocked()
	return nil
}

func (cb *CircuitBreaker) recordFailureLocked() {
	switch cb.state {
	case StateClosed:
		cb.consecutiveFails++
		if cb.consecutiveFails >= cb.config.FailureThreshold {
			cb.state = StateOpen
			cb.lastStateChange = time.Now()
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
		cb.consecutiveSuccess = 0
	case StateOpen:
	}
}

func (cb *CircuitBreaker) recordSuccessLocked() {
	switch cb.state {
	case StateHalfOpen:
		cb.consecutiveSuccess++
		if cb.consecutiveSuccess >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.consecutiveFails = 0
			cb.consecutiveSuccess = 0
			cb.lastStateChange = time.Now()
		}
	case StateClosed:
		cb.consecutiveFails = 0
	case StateOpen:
	}
}
```

### 2. Client Usage with Fallback

```go
func CallRecommendationEngine(cb *CircuitBreaker, client *http.Client) ([]Item, error) {
    var items []Item
    err := cb.Execute(func() error {
        resp, err := client.Get("https://recs.internal/api/v1/recommendations")
        if err != nil {
            return err
        }
        defer resp.Body.Close()

        if resp.StatusCode >= 500 {
            return fmt.Errorf("server error %d", resp.StatusCode)
        }

        return json.NewDecoder(resp.Body).Decode(&items)
    })

    if errors.Is(err, ErrCircuitOpen) {
        // Fallback: return popular items from local cache
        return GetCachedDefaultRecommendations(), nil
    }

    return items, err
}
```
