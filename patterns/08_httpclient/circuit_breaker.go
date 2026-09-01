package httpclient

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker is OPEN: downstream service failing")

// CircuitState represents the current state of the breaker.
type CircuitState string

const (
	StateClosed   CircuitState = "CLOSED"    // Normal traffic
	StateOpen     CircuitState = "OPEN"      // Downstream failure: fast rejection
	StateHalfOpen CircuitState = "HALF_OPEN" // Trial recovery probe
)

// CircuitBreakerConfig defines threshold settings.
type CircuitBreakerConfig struct {
	FailureThreshold int           // Consecutive failures before opening
	SuccessThreshold int           // Consecutive successes in HALF_OPEN before closing
	CooldownTimeout  time.Duration // Time to wait in OPEN before moving to HALF_OPEN
}

// CircuitBreaker protects downstream services from cascading failure.
type CircuitBreaker struct {
	mu               sync.Mutex
	config           CircuitBreakerConfig
	state            CircuitState
	consecutiveFails int
	consecutiveSuccess int
	lastStateChange  time.Time
}

// NewCircuitBreaker creates a circuit breaker starting in CLOSED state.
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

// State returns current state.
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

// Execute wraps an operation with circuit breaker state management.
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
		// Any failure in half-open trips back to open immediately
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
		cb.consecutiveSuccess = 0
	case StateOpen:
		// Already open
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
		// Not possible unless state changed
	}
}

// String status representation
func (cb *CircuitBreaker) String() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return fmt.Sprintf("CircuitBreaker[state=%s fails=%d]", cb.state, cb.consecutiveFails)
}
