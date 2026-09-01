package auth_test

import (
	"errors"
	"testing"

	auth "system-design-patterns/patterns/12_auth"
)

func TestVerifyOwnership(t *testing.T) {
	owner := &auth.AuthenticatedUser{
		ID:    "usr-owner",
		Roles: []string{"member"},
	}

	attacker := &auth.AuthenticatedUser{
		ID:    "usr-attacker",
		Roles: []string{"member"},
	}

	admin := &auth.AuthenticatedUser{
		ID:    "usr-admin",
		Roles: []string{"superadmin"},
	}

	resourceOwnerID := "usr-owner"

	// 1. Owner -> Allowed
	if err := auth.VerifyOwnership(owner, resourceOwnerID); err != nil {
		t.Errorf("expected owner to pass ownership check, got error: %v", err)
	}

	// 2. Attacker -> Denied (IDOR Defense)
	if err := auth.VerifyOwnership(attacker, resourceOwnerID); !errors.Is(err, auth.ErrIDORViolation) {
		t.Errorf("expected ErrIDORViolation for attacker, got: %v", err)
	}

	// 3. Superadmin -> Bypass allowed
	if err := auth.VerifyOwnership(admin, resourceOwnerID, "superadmin"); err != nil {
		t.Errorf("expected superadmin to bypass ownership check, got: %v", err)
	}
}
