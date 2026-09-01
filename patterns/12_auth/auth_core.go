package auth

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated: valid credentials required")
	ErrUnauthorized    = errors.New("unauthorized: permission denied (fail-closed)")
)

// AuthenticatedUser represents identity information verified during Authentication.
type AuthenticatedUser struct {
	ID          string
	Email       string
	Roles       []string
	Permissions map[string]bool
}

type userContextKey struct{}

// WithUser injects AuthenticatedUser into context.
func WithUser(ctx context.Context, u *AuthenticatedUser) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

// GetUser retrieves AuthenticatedUser from context.
func GetUser(ctx context.Context) (*AuthenticatedUser, bool) {
	u, ok := ctx.Value(userContextKey{}).(*AuthenticatedUser)
	return u, ok && u != nil
}

// Authenticator validates identity.
type Authenticator interface {
	Authenticate(r *http.Request) (*AuthenticatedUser, error)
}

// Authorizer evaluates whether identity has permission (Fail-Closed default).
type Authorizer interface {
	Authorize(user *AuthenticatedUser, requiredPermission string) error
}

// FailClosedAuthorizer denies access unless permission is explicitly granted.
type FailClosedAuthorizer struct{}

func (a *FailClosedAuthorizer) Authorize(user *AuthenticatedUser, requiredPermission string) error {
	if user == nil {
		return ErrUnauthenticated
	}

	if user.Permissions == nil || !user.Permissions[requiredPermission] {
		return ErrUnauthorized // Fail-closed: Deny by default
	}

	return nil
}
