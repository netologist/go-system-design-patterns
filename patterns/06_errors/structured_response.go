package errorspattern

import (
	"encoding/json"
	"net/http"
	"time"
)

// FieldViolation details an invalid input field.
type FieldViolation struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// ProblemDetails implements RFC 7807 Problem Details specification for HTTP APIs.
type ProblemDetails struct {
	Type          string           `json:"type"`                     // URI reference identifying problem type
	Title         string           `json:"title"`                    // Short, human-readable summary
	Status        int              `json:"status"`                   // HTTP status code
	Detail        string           `json:"detail,omitempty"`         // Specific explanation of this occurrence
	Instance      string           `json:"instance,omitempty"`       // URI reference identifying the specific occurrence
	CorrelationID string           `json:"correlation_id,omitempty"` // Tracing identifier
	Timestamp     time.Time        `json:"timestamp"`
	InvalidParams []FieldViolation `json:"invalid_params,omitempty"` // Validation breakdown
}

// WriteProblemDetails writes the ProblemDetails struct as application/problem+json.
func WriteProblemDetails(w http.ResponseWriter, problem ProblemDetails) {
	if problem.Timestamp.IsZero() {
		problem.Timestamp = time.Now().UTC()
	}
	if problem.Type == "" {
		problem.Type = "about:blank"
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}
