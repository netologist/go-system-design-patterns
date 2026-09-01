package errorspattern

import (
	"errors"
	"fmt"
)

// Predefined Sentinel Errors for domain classification.
var (
	ErrNotFound       = errors.New("resource not found")
	ErrConflict       = errors.New("resource conflict")
	ErrUnauthorized   = errors.New("unauthorized access")
	ErrForbidden      = errors.New("forbidden operation")
	ErrValidation     = errors.New("validation failed")
	ErrInternalServer = errors.New("internal server error")
)

// WrapOperation wraps an underlying error with an operational context using %w.
func WrapOperation(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

// FindUserByID simulates repository returning wrapped sentinel error.
func FindUserByID(id string) error {
	if id == "" {
		return WrapOperation("FindUserByID", ErrValidation)
	}
	if id == "missing" {
		return WrapOperation("FindUserByID", ErrNotFound)
	}
	if id == "conflict" {
		return WrapOperation("FindUserByID", ErrConflict)
	}
	return nil
}
