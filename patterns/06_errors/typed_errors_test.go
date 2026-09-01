package errorspattern_test

import (
	"errors"
	"fmt"
	"testing"

	errorspattern "system-design-patterns/patterns/06_errors"
)

func TestDomainError_ExtractionWithErrorsAs(t *testing.T) {
	rootDBErr := errors.New("sql: no rows in result set")
	domainErr := errorspattern.NewNotFoundError("UserService.Get", "User", rootDBErr)

	// Wrap inside middleware / handler
	wrappedErr := fmt.Errorf("HTTPHandler.HandleGet: %w", domainErr)

	extracted, ok := errorspattern.ExtractDomainError(wrappedErr)
	if !ok {
		t.Fatalf("failed to extract DomainError using errors.As")
	}

	if extracted.Kind != errorspattern.KindNotFound {
		t.Errorf("expected KindNotFound, got: %s", extracted.Kind)
	}

	if extracted.Op != "UserService.Get" {
		t.Errorf("expected Op 'UserService.Get', got: %s", extracted.Op)
	}

	// Verify unwrap reaches rootDBErr
	if !errors.Is(wrappedErr, rootDBErr) {
		t.Errorf("expected wrapped error chain to match rootDBErr")
	}
}

func TestDomainError_ValidationError(t *testing.T) {
	vErr := errorspattern.NewValidationError("OrderService.Validate", "email", "must be a valid email address")

	if vErr.Kind != errorspattern.KindValidation {
		t.Errorf("expected KindValidation, got: %s", vErr.Kind)
	}

	if vErr.Field != "email" {
		t.Errorf("expected field 'email', got: %s", vErr.Field)
	}
}
