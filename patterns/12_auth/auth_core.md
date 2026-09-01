# Authentication & Fail-Closed Authorization Core

## 1. Overview & Concept

In enterprise Go backends, managing identity and permissions requires a strict separation of concerns between **Authentication (AuthN)**—verifying *who* the caller is—and **Authorization (AuthZ)**—determining *what* actions the verified caller is permitted to perform.

The **Auth Core** pattern defines foundational abstractions, domain models, and defensive primitives for security:
1. **Separation of AuthN and AuthZ Contracts:** Defining distinct `Authenticator` and `Authorizer` interfaces.
2. **Fail-Closed Default Authorization:** Implementing `FailClosedAuthorizer`, which rejects access by default unless an explicit permission grant is positively matched.
3. **Type-Safe Context Propagation:** Safely attaching and retrieving the verified `AuthenticatedUser` from `context.Context` using unexported struct keys.
4. **Standardized Sentinel Errors:** Providing unambiguous domain errors (`ErrUnauthenticated` for 401 scenarios and `ErrUnauthorized` for 403 scenarios).

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without a formalized, fail-closed authorization core:

* **Fail-Open Security Vulnerabilities:** In poorly designed systems, encountering a `nil` permission map, missing role, or unhandled error state inadvertently allows requests to pass through ("failing open"). Attackers exploit unhandled default cases to escalate privileges.
* **Context Key Collisions & Type Assertion Panics:** Storing user information in `r.Context()` with string keys (e.g. `ctx.Value("user")`) risks silent key collisions with third-party packages or runtime panics if unexpected types are cast.
* **MIME/Status Code Confusion (401 vs. 403):** Conflating unauthenticated requests (missing/expired credentials, requiring HTTP 401) with unauthorized requests (valid credentials but lacking permissions, requiring HTTP 403) breaks client-side token refresh workflows.
* **Coupling Business Logic to Token Formats:** When handlers parse raw JWTs or session cookies directly, refactoring from session cookies to OAuth2/OIDC requires rewriting every endpoint in the application.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The security pipeline enforces a two-stage evaluation gate: identity extraction followed by fail-closed authorization.

```
                      Incoming HTTP Request
                               │
                               ▼
                   ┌───────────────────────┐
                   │     Authenticator     │ ◄── Stage 1: AuthN (Identity Verification)
                   │ r.Header["Bearer..."] │
                   └───────────┬───────────┘
                               │
                 ┌─────────────┴─────────────┐
                 │                           │
          [Invalid / Expired]            [Valid Identity]
                 │                           │
                 ▼                           ▼
          ErrUnauthenticated        Inject WithUser(ctx, u)
          (HTTP 401 Challenge)               │
                                             ▼
                               ┌───────────────────────────┐
                               │   FailClosedAuthorizer    │ ◄── Stage 2: AuthZ (Permission Gate)
                               │ Check u.Permissions[perm] │
                               └─────────────┬─────────────┘
                                             │
                              ┌──────────────┴──────────────┐
                              │                             │
                       [Perm Missing]                [Perm Granted]
                              │                             │
                              ▼                             ▼
                      ErrUnauthorized               next.ServeHTTP(w, r)
                      (HTTP 403 Forbidden)          (Execute business handler)
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Fail-Closed by Design
The `FailClosedAuthorizer` explicitly checks:
- If `user == nil`: Returns `ErrUnauthenticated`.
- If `user.Permissions == nil`: Returns `ErrUnauthorized`.
- If `user.Permissions[requiredPermission] == false`: Returns `ErrUnauthorized`.
No execution branch allows access unless the required permission is explicitly set to `true`.

### 2. Context Key Protection
Go's `context.WithValue` accepts `any` as key and value. Using an unexported struct:
```go
type userContextKey struct{}
```
guarantees that no external package can overwrite, read, or collide with the authenticated user context object.

### 3. Separation of Identity from Presentation
The `AuthenticatedUser` struct contains clean identity primitives (`ID`, `Email`, `Roles`, `Permissions`). Domain handlers consume this struct without knowing whether the identity originated from a JWT, an mTLS client certificate, an API key, or an OIDC session cookie.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/12_auth/auth_core.go`)

```go
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
```

### 2. Middleware Adapter Example

```go
func RequirePermission(authorizer auth.Authorizer, permission string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, _ := auth.GetUser(r.Context())
            
            if err := authorizer.Authorize(user, permission); err != nil {
                if errors.Is(err, auth.ErrUnauthenticated) {
                    http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
                    return
                }
                http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```
