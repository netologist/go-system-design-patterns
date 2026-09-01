package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "system-design-patterns/patterns/11_middleware"
)

func TestMiddlewareChain_ExecutionOrder(t *testing.T) {
	var executionTrace []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionTrace = append(executionTrace, "M1_ENTER")
			next.ServeHTTP(w, r)
			executionTrace = append(executionTrace, "M1_EXIT")
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionTrace = append(executionTrace, "M2_ENTER")
			next.ServeHTTP(w, r)
			executionTrace = append(executionTrace, "M2_EXIT")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionTrace = append(executionTrace, "HANDLER")
		w.WriteHeader(http.StatusOK)
	})

	chain := middleware.NewChain(m1, m2)
	finalHandler := chain.Then(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	finalHandler.ServeHTTP(w, req)

	expected := []string{"M1_ENTER", "M2_ENTER", "HANDLER", "M2_EXIT", "M1_EXIT"}
	if len(executionTrace) != len(expected) {
		t.Fatalf("expected trace %v, got %v", expected, executionTrace)
	}

	for i, v := range expected {
		if executionTrace[i] != v {
			t.Errorf("at index %d: expected %s, got %s", i, v, executionTrace[i])
		}
	}
}
