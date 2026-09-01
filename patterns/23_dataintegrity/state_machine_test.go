package dataintegrity_test

import (
	"errors"
	"testing"

	dataintegrity "system-design-patterns/patterns/23_dataintegrity"
)

func TestPaymentStateMachine_ValidAndInvalidTransitions(t *testing.T) {
	sm := dataintegrity.NewPaymentStateMachine(dataintegrity.StatePending)

	// 1. Valid: Pending -> Processing
	err := sm.TransitionTo(dataintegrity.StateProcessing)
	if err != nil {
		t.Fatalf("transition to processing failed: %v", err)
	}

	// 2. Valid: Processing -> Succeeded
	err = sm.TransitionTo(dataintegrity.StateSucceeded)
	if err != nil {
		t.Fatalf("transition to succeeded failed: %v", err)
	}

	// 3. Invalid: Succeeded -> Processing (Disallowed rollback)
	err = sm.TransitionTo(dataintegrity.StateProcessing)
	if !errors.Is(err, dataintegrity.ErrInvalidStateTransition) {
		t.Errorf("expected ErrInvalidStateTransition, got: %v", err)
	}

	// 4. Valid: Succeeded -> Refunded
	err = sm.TransitionTo(dataintegrity.StateRefunded)
	if err != nil {
		t.Fatalf("transition to refunded failed: %v", err)
	}

	// 5. Invalid: Terminal Refunded -> Processing
	err = sm.TransitionTo(dataintegrity.StateProcessing)
	if !errors.Is(err, dataintegrity.ErrInvalidStateTransition) {
		t.Errorf("expected error transitioning from terminal state, got: %v", err)
	}
}
