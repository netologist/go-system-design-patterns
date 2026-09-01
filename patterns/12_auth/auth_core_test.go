package auth_test

import (
	"errors"
	"testing"

	auth "system-design-patterns/patterns/12_auth"
)

func TestFailClosedAuthorizer(t *testing.T) {
	authorizer := &auth.FailClosedAuthorizer{}

	// 1. Nil user -> Unauthenticated
	if err := authorizer.Authorize(nil, "order:write"); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("expected ErrUnauthenticated for nil user, got: %v", err)
	}

	// 2. User without required permission -> Unauthorized
	user := &auth.AuthenticatedUser{
		ID:          "usr-1",
		Permissions: map[string]bool{"order:read": true},
	}
	if err := authorizer.Authorize(user, "order:write"); !errors.Is(err, auth.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized for missing permission, got: %v", err)
	}

	// 3. User with permission -> Allowed
	user.Permissions["order:write"] = true
	if err := authorizer.Authorize(user, "order:write"); err != nil {
		t.Errorf("expected permission granted, got error: %v", err)
	}
}
