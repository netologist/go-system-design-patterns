package errorspattern_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	errorspattern "system-design-patterns/patterns/06_errors"
)

func TestErrorMapper_DomainErrorMapping(t *testing.T) {
	mapper := errorspattern.NewErrorMapper()

	// 1. Not Found
	status, pub := mapper.MapToHTTP(errorspattern.NewNotFoundError("Get", "User", nil))
	if status != http.StatusNotFound || pub.Code != "RESOURCE_NOT_FOUND" {
		t.Errorf("expected 404 RESOURCE_NOT_FOUND, got: %d %s", status, pub.Code)
	}

	// 2. Validation
	status, pub = mapper.MapToHTTP(errorspattern.NewValidationError("Create", "email", "invalid email"))
	if status != http.StatusUnprocessableEntity || pub.Field != "email" {
		t.Errorf("expected 422 with field email, got: %d %s (field=%s)", status, pub.Code, pub.Field)
	}
}

func TestErrorMapper_InternalDetailsNeverLeaked(t *testing.T) {
	mapper := errorspattern.NewErrorMapper()

	// Internal raw database error containing password/connection details
	rawDBErr := errors.New("pq: password authentication failed for user 'postgres' on 10.0.0.5:5432")

	status, pub := mapper.MapToHTTP(rawDBErr)
	if status != http.StatusInternalServerError {
		t.Errorf("expected 500 status code, got: %d", status)
	}

	if strings.Contains(pub.Message, "password") || strings.Contains(pub.Message, "postgres") || strings.Contains(pub.Message, "10.0.0.5") {
		t.Fatalf("CRITICAL SECURITY FLAW: database credentials leaked in public error payload: %s", pub.Message)
	}
}
