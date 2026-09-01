package auth

import (
	"errors"
	"strings"
)

var (
	ErrIDORViolation = errors.New("access denied: you do not own this resource")
)

// VerifyOwnership enforces that the requesting user owns the resource or has an administrative bypass role.
func VerifyOwnership(user *AuthenticatedUser, resourceOwnerID string, bypassRoles ...string) error {
	if user == nil {
		return ErrUnauthenticated
	}

	// 1. Direct owner match
	if user.ID == resourceOwnerID {
		return nil
	}

	// 2. Check administrative bypass roles (e.g. "superadmin", "support")
	for _, userRole := range user.Roles {
		for _, bypassRole := range bypassRoles {
			if strings.EqualFold(userRole, bypassRole) {
				return nil
			}
		}
	}

	// 3. Reject access (IDOR defense)
	return ErrIDORViolation
}
