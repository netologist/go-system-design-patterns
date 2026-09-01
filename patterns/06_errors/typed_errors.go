package errorspattern

import (
	"errors"
	"fmt"
)

// ErrorKind classifies domain errors into logical taxonomy categories.
type ErrorKind string

const (
	KindNotFound     ErrorKind = "NOT_FOUND"
	KindConflict     ErrorKind = "CONFLICT"
	KindValidation   ErrorKind = "VALIDATION"
	KindUnauthorized ErrorKind = "UNAUTHORIZED"
	KindForbidden    ErrorKind = "FORBIDDEN"
	KindInternal     ErrorKind = "INTERNAL"
)

// DomainError provides structured, typed metadata about a failure.
type DomainError struct {
	Kind    ErrorKind
	Op      string
	Message string
	Field   string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("[%s] %s: field '%s' - %s", e.Kind, e.Op, e.Field, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %s (caused by: %v)", e.Kind, e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Kind, e.Op, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// Helper constructors
func NewNotFoundError(op, resource string, err error) *DomainError {
	return &DomainError{
		Kind:    KindNotFound,
		Op:      op,
		Message: fmt.Sprintf("%s not found", resource),
		Err:     err,
	}
}

func NewValidationError(op, field, message string) *DomainError {
	return &DomainError{
		Kind:    KindValidation,
		Op:      op,
		Field:   field,
		Message: message,
	}
}

func NewConflictError(op, message string, err error) *DomainError {
	return &DomainError{
		Kind:    KindConflict,
		Op:      op,
		Message: message,
		Err:     err,
	}
}

func NewInternalError(op string, err error) *DomainError {
	return &DomainError{
		Kind:    KindInternal,
		Op:      op,
		Message: "an internal server error occurred",
		Err:     err,
	}
}

// ExtractDomainError extracts *DomainError from any error chain using errors.As.
func ExtractDomainError(err error) (*DomainError, bool) {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr, true
	}
	return nil, false
}
