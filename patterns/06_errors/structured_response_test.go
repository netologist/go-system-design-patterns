package errorspattern_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	errorspattern "system-design-patterns/patterns/06_errors"
)

func TestWriteProblemDetails(t *testing.T) {
	w := httptest.NewRecorder()

	problem := errorspattern.ProblemDetails{
		Type:          "https://example.com/probs/validation",
		Title:         "Your request parameters didn't validate.",
		Status:        http.StatusBadRequest,
		Detail:        "The payload was missing required fields.",
		Instance:      "/api/v1/orders/123",
		CorrelationID: "corr-9876",
		InvalidParams: []errorspattern.FieldViolation{
			{Field: "amount", Reason: "must be positive integer"},
		},
	}

	errorspattern.WriteProblemDetails(w, problem)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400, got: %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got: %s", contentType)
	}

	var decoded errorspattern.ProblemDetails
	if err := json.NewDecoder(w.Body).Decode(&decoded); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if decoded.CorrelationID != "corr-9876" || len(decoded.InvalidParams) != 1 {
		t.Errorf("decoded problem mismatch: %+v", decoded)
	}
}
