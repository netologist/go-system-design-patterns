package errorspattern_test

import (
	"errors"
	"fmt"
	"testing"

	errorspattern "system-design-patterns/patterns/06_errors"
)

func TestSentinelWrapping_ErrorsIs(t *testing.T) {
	err := errorspattern.FindUserByID("missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Wrap in another service layer
	serviceErr := fmt.Errorf("UserService.GetUser: %w", err)

	// Verify errors.Is detects the root sentinel error
	if !errors.Is(serviceErr, errorspattern.ErrNotFound) {
		t.Errorf("expected errors.Is to match ErrNotFound in wrapped chain: %v", serviceErr)
	}

	// Should not match ErrConflict
	if errors.Is(serviceErr, errorspattern.ErrConflict) {
		t.Errorf("errors.Is incorrectly matched ErrConflict")
	}
}

func TestSentinelWrapping_NoStringMatching(t *testing.T) {
	err := errorspattern.FindUserByID("conflict")
	serviceErr := errorspattern.WrapOperation("AccountService.Create", err)

	// In production, NEVER do: if serviceErr.Error() == "resource conflict"
	// Always use errors.Is
	if !errors.Is(serviceErr, errorspattern.ErrConflict) {
		t.Fatalf("expected errors.Is match on ErrConflict")
	}
}
