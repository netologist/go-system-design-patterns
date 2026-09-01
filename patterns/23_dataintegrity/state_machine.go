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
	mu            sync.Mutex
	currentState  PaymentState
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
