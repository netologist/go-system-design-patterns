package auth_test

import (
	"context"
	"testing"

	auth "system-design-patterns/patterns/12_auth"
)

func TestRBACRegistry_RoleHierarchy(t *testing.T) {
	rbac := auth.NewRBACRegistry()

	// Admin has all
	if !rbac.HasPermission([]string{"admin"}, "document:delete") {
		t.Error("expected admin to have document:delete")
	}

	// Editor has write but not delete
	if !rbac.HasPermission([]string{"editor"}, "document:write") {
		t.Error("expected editor to have document:write")
	}
	if rbac.HasPermission([]string{"editor"}, "document:delete") {
		t.Error("editor should not have document:delete")
	}

	// Viewer has read only
	if !rbac.HasPermission([]string{"viewer"}, "document:read") {
		t.Error("expected viewer to have document:read")
	}
	if rbac.HasPermission([]string{"viewer"}, "document:write") {
		t.Error("viewer should not have document:write")
	}
}

func TestABACAuthorizer_DynamicAttributes(t *testing.T) {
	abac := auth.NewABACAuthorizer()
	ctx := context.Background()

	// Non-confidential -> Allowed
	err := abac.Evaluate(ctx, auth.ABACContext{
		SubjectID:      "u-1",
		ResourceOwnerID: "u-2",
		IsConfidential: false,
	})
	if err != nil {
		t.Errorf("expected non-confidential allowed, got: %v", err)
	}

	// Confidential, not owner -> Denied
	err = abac.Evaluate(ctx, auth.ABACContext{
		SubjectID:      "u-1",
		ResourceOwnerID: "u-2",
		IsConfidential: true,
		Department:     "FINANCE",
	})
	if err == nil {
		t.Error("expected non-owner access to confidential doc to be denied")
	}

	// Confidential, owner in FINANCE -> Allowed
	err = abac.Evaluate(ctx, auth.ABACContext{
		SubjectID:      "u-1",
		ResourceOwnerID: "u-1",
		IsConfidential: true,
		Department:     "FINANCE",
	})
	if err != nil {
		t.Errorf("expected owner in FINANCE to be allowed, got: %v", err)
	}
}
