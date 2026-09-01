package httpclient_test

import (
	"testing"

	httpclient "system-design-patterns/patterns/08_httpclient"
)

func TestRetryBudget_ExhaustionAndReplenish(t *testing.T) {
	// 10% ratio, initial max 5 tokens, 0 baseline replenishment for test control
	budget := httpclient.NewRetryBudget(0.10, 5, 0.0001)

	// Consume all 5 initial tokens
	for i := range 5 {
		if !budget.TryAcquireRetry() {
			t.Fatalf("expected token %d to be available", i+1)
		}
	}

	// 6th attempt should be rejected (budget exhausted)
	if budget.TryAcquireRetry() {
		t.Fatal("expected retry to be rejected after exhausting budget")
	}

	// Record 10 regular requests -> earns 10 * 0.10 = 1.0 token
	for range 10 {
		budget.RecordRequest()
	}

	// Now 1 retry token should be granted
	if !budget.TryAcquireRetry() {
		t.Fatal("expected 1 retry token to be available after 10 regular requests")
	}

	// Next should be exhausted again
	if budget.TryAcquireRetry() {
		t.Fatal("expected budget to be exhausted again")
	}
}
