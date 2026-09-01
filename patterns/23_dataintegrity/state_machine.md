# Finite State Machine (FSM) Pattern

## Overview & Definition
The **Finite State Machine (FSM) Pattern** enforces deterministic lifecycle progression and prevents illegal state transitions across core business entities (such as payments, order fulfillment, subscription statuses, and deployment jobs).

In complex domain models, an entity moves through a finite set of discrete states (e.g., `PENDING -> PROCESSING -> SUCCEEDED`). Without an explicit state machine, concurrent webhooks, asynchronous background retries, and out-of-order API calls can transition an entity into contradictory or invalid states (e.g., executing a refund on a payment that is still in `PENDING` state, or marking a `FAILED` order as `SUCCEEDED`).

The `PaymentStateMachine` encapsulates allowable transition graphs, guards transitions with mutex synchronization, and rejects illegal state jumps with descriptive domain errors.

---

## Problem Statement
Relying on ad-hoc boolean flags (`isPaid`, `isRefunded`) or unconstrained status string updates leads to severe data corruption and financial discrepancies.

### Failure Scenarios Without This Pattern
- **Double-Fulfillment / Out-of-Order Webhooks:** A payment gateway sends a `charge.failed` webhook followed by a delayed `charge.succeeded` webhook due to network jitter. Without transition rules, a failed order could be marked as succeeded without valid payment.
- **Illegal State Transitions:** An automated cron job attempts to process a refund against an order that never succeeded (`PENDING -> REFUNDED`), corrupting financial ledgers.
- **Zombie State Mutations:** Modifying records that have already reached terminal states (`REFUNDED` or `FAILED`).
- **Concurrent Race Conditions:** Two concurrent worker goroutines attempt to process and cancel an order simultaneously, resulting in a conflicting split-brain state.

---

## Architectural Mechanism & Flow
The `PaymentStateMachine` maintains an internal transition matrix mapping each state to its allowed destination states:

```
                      ┌────────────────────────┐
                      │    State: PENDING      │
                      └───────────┬────────────┘
                                  │
                    ┌─────────────┴─────────────┐
                    ▼                           ▼
      ┌───────────────────────────┐   ┌───────────────────────────┐
      │     State: PROCESSING     │   │       State: FAILED       │
      └─────────────┬─────────────┘   │      (Terminal State)     │
                    │                 └───────────────────────────┘
          ┌─────────┴─────────┐
          ▼                   ▼
┌───────────────────┐   ┌───────────────────┐
│ State: SUCCEEDED  │   │   State: FAILED   │
└─────────┬─────────┘   │  (Terminal State) │
          │             └───────────────────┘
          ▼
┌───────────────────┐
│  State: REFUNDED  │
│ (Terminal State)  │
└───────────────────┘
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Explicit Terminal States:** Define terminal states (states with empty allowed-transition sets) so completed transactions can never be mutated again.
- **Combine In-Memory FSM with Database State Checks:** Always execute state transitions in relational databases using optimistic concurrency control (`UPDATE payments SET status = 'PROCESSING' WHERE id = $1 AND status = 'PENDING'`).
- **Emit Transition Events:** On every successful transition, emit domain events (`PaymentTransitionedEvent{From, To, Timestamp}`) to trigger downstream notification workers, outbox publishers, and analytics.
- **Thread-Safe Transition Method:** Protect in-memory FSM state using `sync.Mutex` so concurrent goroutines cannot corrupt intermediate state.

### Pitfalls to Avoid
- **Implicit Transitions:** Allowing transitions to occur automatically without explicit validation calls.
- **Boolean Flag Proliferation:** Using multiple flags (`isPending`, `isProcessing`, `isDone`, `isCancelled`) instead of a single authoritative enum state.
- **Silent Failures:** Swallowing transition errors rather than propagating `ErrInvalidStateTransition` up to API callers or webhook consumers.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `state_machine.go` enforces the payment state transition matrix:

```go
package dataintegrity

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidStateTransition = errors.New("invalid state transition")
)

type PaymentState string

const (
	StatePending    PaymentState = "PENDING"
	StateProcessing PaymentState = "PROCESSING"
	StateSucceeded  PaymentState = "SUCCEEDED"
	StateFailed     PaymentState = "FAILED"
	StateRefunded   PaymentState = "REFUNDED"
)

// PaymentStateMachine enforces valid lifecycle transitions on payment objects.
type PaymentStateMachine struct {
	mu                 sync.Mutex
	currentState       PaymentState
	allowedTransitions map[PaymentState]map[PaymentState]bool
}

func NewPaymentStateMachine(initialState PaymentState) *PaymentStateMachine {
	if initialState == "" {
		initialState = StatePending
	}

	return &PaymentStateMachine{
		currentState: initialState,
		allowedTransitions: map[PaymentState]map[PaymentState]bool{
			StatePending: {
				StateProcessing: true,
				StateFailed:     true,
			},
			StateProcessing: {
				StateSucceeded: true,
				StateFailed:    true,
			},
			StateSucceeded: {
				StateRefunded: true,
			},
			StateFailed:   {}, // Terminal state
			StateRefunded: {}, // Terminal state
		},
	}
}

// TransitionTo validates and applies a transition to targetState.
func (sm *PaymentStateMachine) TransitionTo(targetState PaymentState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	allowed := sm.allowedTransitions[sm.currentState]
	if !allowed[targetState] {
		return fmt.Errorf("%w: cannot transition from %s to %s",
			ErrInvalidStateTransition, sm.currentState, targetState)
	}

	sm.currentState = targetState
	return nil
}

func (sm *PaymentStateMachine) CurrentState() PaymentState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.currentState
}
```

### Production Payment Processing Example

```go
func ProcessRefund(paymentID string, fsm *dataintegrity.PaymentStateMachine) error {
    // Validate transition
    if err := fsm.TransitionTo(dataintegrity.StateRefunded); err != nil {
        return fmt.Errorf("cannot refund payment %s: %w", paymentID, err)
    }

    // Execute refund through payment gateway...
    log.Printf("[PAYMENT] Payment %s successfully transitioned to %s", paymentID, fsm.CurrentState())
    return nil
}
```
