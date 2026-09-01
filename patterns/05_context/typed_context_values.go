package contextpattern

import (
	"context"
)

// Unexported key types prevent collision across packages.
type (
	requestIDKey struct{}
	tenantIDKey  struct{}
	userKey      struct{}
)

// UserPrincipal represents authenticated user information in context.
type UserPrincipal struct {
	UserID string
	Role   string
	Email  string
}

// WithRequestID stores a request ID in the context safely.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey{}).(string); ok {
		return val
	}
	return ""
}

// WithTenantID stores tenant isolation ID in the context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

// GetTenantID retrieves tenant ID from context.
func GetTenantID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(tenantIDKey{}).(string)
	return val, ok
}

// WithUserPrincipal stores authenticated user in context.
func WithUserPrincipal(ctx context.Context, user *UserPrincipal) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// GetUserPrincipal retrieves authenticated user from context.
func GetUserPrincipal(ctx context.Context) (*UserPrincipal, bool) {
	val, ok := ctx.Value(userKey{}).(*UserPrincipal)
	return val, ok && val != nil
}
