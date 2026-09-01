package httpserver_test

import (
	"errors"
	"strings"
	"testing"

	httpserver "system-design-patterns/patterns/07_httpserver"
)

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestDecodeStrictJSON_Valid(t *testing.T) {
	raw := `{"name": "Hasan", "email": "hasan@example.com"}`
	var req CreateUserRequest

	err := httpserver.DecodeStrictJSON(strings.NewReader(raw), &req)
	if err != nil {
		t.Fatalf("expected successful decode, got: %v", err)
	}

	if req.Name != "Hasan" || req.Email != "hasan@example.com" {
		t.Errorf("decoded struct mismatch: %+v", req)
	}
}

func TestDecodeStrictJSON_UnknownFieldRejected(t *testing.T) {
	// Client typo: "emali" instead of "email"
	raw := `{"name": "Hasan", "emali": "hasan@example.com"}`
	var req CreateUserRequest

	err := httpserver.DecodeStrictJSON(strings.NewReader(raw), &req)
	if err == nil {
		t.Fatal("expected error on unknown field, got nil")
	}
	if !errors.Is(err, httpserver.ErrUnknownField) {
		t.Errorf("expected ErrUnknownField, got: %v", err)
	}
}

func TestDecodeStrictJSON_MultipleJSONObjectsRejected(t *testing.T) {
	raw := `{"name": "User1", "email": "u1@example.com"}{"name": "User2", "email": "u2@example.com"}`
	var req CreateUserRequest

	err := httpserver.DecodeStrictJSON(strings.NewReader(raw), &req)
	if err == nil {
		t.Fatal("expected error on multiple JSON documents, got nil")
	}
	if !errors.Is(err, httpserver.ErrMultipleJSONVals) {
		t.Errorf("expected ErrMultipleJSONVals, got: %v", err)
	}
}

func TestDecodeStrictJSON_EmptyBody(t *testing.T) {
	var req CreateUserRequest
	err := httpserver.DecodeStrictJSON(strings.NewReader(""), &req)
	if !errors.Is(err, httpserver.ErrEmptyBody) {
		t.Errorf("expected ErrEmptyBody, got: %v", err)
	}
}
