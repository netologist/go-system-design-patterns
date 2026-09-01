package auth

import (
	"context"
	"errors"
	"strings"
)

// Role represents a predefined authorization role.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// RBACRegistry manages role-to-permission mappings.
type RBACRegistry struct {
	rolePermissions map[Role]map[string]bool
}

func NewRBACRegistry() *RBACRegistry {
	return &RBACRegistry{
		rolePermissions: map[Role]map[string]bool{
			RoleAdmin: {
				"document:read":   true,
				"document:write":  true,
				"document:delete": true,
			},
			RoleEditor: {
				"document:read":  true,
				"document:write": true,
			},
			RoleViewer: {
				"document:read": true,
			},
		},
	}
}

// HasPermission checks if any of the user's roles grant the requested permission.
func (r *RBACRegistry) HasPermission(roles []string, permission string) bool {
	for _, roleStr := range roles {
		role := Role(strings.ToLower(roleStr))
		if perms, ok := r.rolePermissions[role]; ok {
			if perms[permission] {
				return true
			}
		}
	}
	return false
}

// ABACContext holds contextual runtime attributes for dynamic evaluation.
type ABACContext struct {
	SubjectID      string
	ResourceOwnerID string
	Department     string
	IsConfidential bool
}

// ABACAuthorizer evaluates fine-grained dynamic attributes.
type ABACAuthorizer struct{}

func NewABACAuthorizer() *ABACAuthorizer {
	return &ABACAuthorizer{}
}

// Evaluate evaluates access based on dynamic attributes.
// Rule: A user can access a confidential document ONLY IF they are the owner AND in the same department.
func (a *ABACAuthorizer) Evaluate(ctx context.Context, abacCtx ABACContext) error {
	if !abacCtx.IsConfidential {
		return nil // Non-confidential allowed
	}

	if abacCtx.SubjectID != abacCtx.ResourceOwnerID {
		return errors.New("abac access denied: must be resource owner for confidential documents")
	}

	if abacCtx.Department != "FINANCE" && abacCtx.Department != "EXECUTIVE" {
		return errors.New("abac access denied: department clearance insufficient")
	}

	return nil
}
