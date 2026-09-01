package contextpattern_test

import (
	"context"
	"testing"

	contextpattern "system-design-patterns/patterns/05_context"
)

func TestTypedContextValues_RequestID(t *testing.T) {
	ctx := context.Background()

	// Missing
	if id := contextpattern.GetRequestID(ctx); id != "" {
		t.Errorf("expected empty string for missing request ID, got: %s", id)
	}

	// Present
	ctx = contextpattern.WithRequestID(ctx, "req-12345")
	if id := contextpattern.GetRequestID(ctx); id != "req-12345" {
		t.Errorf("expected req-12345, got: %s", id)
	}
}

func TestTypedContextValues_TenantID(t *testing.T) {
	ctx := context.Background()

	if _, ok := contextpattern.GetTenantID(ctx); ok {
		t.Error("expected ok=false for missing tenant ID")
	}

	ctx = contextpattern.WithTenantID(ctx, "tenant-acme")
	tenant, ok := contextpattern.GetTenantID(ctx)
	if !ok || tenant != "tenant-acme" {
		t.Errorf("expected tenant-acme, got %s (ok=%v)", tenant, ok)
	}
}

func TestTypedContextValues_UserPrincipal(t *testing.T) {
	ctx := context.Background()

	if _, ok := contextpattern.GetUserPrincipal(ctx); ok {
		t.Error("expected ok=false for missing user principal")
	}

	user := &contextpattern.UserPrincipal{
		UserID: "usr-9",
		Role:   "admin",
		Email:  "admin@acme.com",
	}

	ctx = contextpattern.WithUserPrincipal(ctx, user)
	retrieved, ok := contextpattern.GetUserPrincipal(ctx)
	if !ok || retrieved.UserID != "usr-9" || retrieved.Role != "admin" {
		t.Errorf("user principal mismatch: %+v (ok=%v)", retrieved, ok)
	}
}
