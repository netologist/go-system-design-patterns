# Response Envelope, Pagination Metadata, and Output Filtering

## Overview & Definition

Building enterprise-grade APIs requires strict consistency across all responses. Without standardized structures, frontends and third-party consumers must write custom parsers for every individual endpoint.

The **Response Envelope and Output Filtering** pattern establishes three critical API design standards:
1. **Generic Envelope (`APIResponse[T]`):** Wraps all JSON responses in a unified top-level structure containing `data`, `meta`, and `error` keys.
2. **Standardized Pagination Metadata (`APIMeta`):** Provides collection pagination context (`page`, `limit`, `total_items`).
3. **Strict Domain-to-DTO Output Filtering (`FilterUserOutput`):** Transforms internal database entities into safe, public DTOs, stripping sensitive attributes (e.g. `PasswordHash`, `SSN`) before serialization.

---

## Problem Statement

Ad-hoc JSON serialization in backend endpoints introduces major security and operational risks:

* **Sensitive Data Exposure (CWE-200 / CWE-359):** Serializing internal database structs directly to JSON (e.g. `json.NewEncoder(w).Encode(user)`) often exposes hashed passwords, secret tokens, internal status flags, or PII (Social Security Numbers, billing details) if `json:"-"` tags are forgotten or refactored away.
* **Inconsistent Response Shapes:** Returning bare arrays (`[...]`) on one endpoint, raw objects (`{...}`) on another, and nested envelopes on a third increases client integration complexity and breaks automated client SDK generation.
* **Missing Pagination Context:** Returning a bare list without total count metadata makes it impossible for client UIs to render page selectors or calculate total pages.

---

## Architectural Mechanism & Flow

```
                      Internal Database Layer
                      [UserAccount Entity: ID, Email, PasswordHash, SSN, CreatedAt]
                                     |
                                     v
                      +------------------------------------------+
                      |         FilterUserOutput(u)              |
                      |  Maps entity -> UserAccountResponseDTO   |
                      |  (Strips PasswordHash & SSN)             |
                      +------------------------------------------+
                                     |
                                     v
                      +------------------------------------------+
                      | WriteSuccessResponse(w, 200, dto, meta)  |
                      +------------------------------------------+
                                     |
                                     v
                      +------------------------------------------+
                      | Output JSON Wire Format:                 |
                      | {                                        |
                      |   "data": {                              |
                      |     "id": "usr_123",                     |
                      |     "email": "hasan@example.com",        |
                      |     "created_at": "2026-08-29T10:00:00Z" |
                      |   },                                     |
                      |   "meta": {                              |
                      |     "page": 1,                           |
                      |     "limit": 20,                         |
                      |     "total_items": 150                   |
                      |   }                                      |
                      | }                                        |
                      +------------------------------------------+
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Never Serialize Database Models Directly:** Always define explicit `ResponseDTO` structs. Explicit field mapping in a `Filter*` function acts as a compile-time security firewall against data leaks.
* **Use Generics for Envelope Types:** `APIResponse[T any]` enables type-safe compile-time envelopes without resorting to `any` / `interface{}` casting.
* **Canonicalize User Input:** Apply normalization helpers (e.g. `NormalizeEmail(email)`) before database queries or serialization.

### Common Pitfalls
* **Relying Exclusively on `json:"-"` Tags:** Adding new fields to database models will automatically expose them in API responses if developers forget to add `json:"-"` tags. Explicit DTO mapping eliminates this failure mode.
* **Empty Slices Serializing as `null`:** In Go, an uninitialized slice (`var users []UserDTO`) serializes to `null` in JSON. Initialize empty slices (`users := make([]UserDTO, 0)`) so the JSON output is `[]`.

---

## Code Walkthrough & Usage

### 1. Implementation (`response_envelope.go`)

```go
package apidesign

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// APIResponse represents standard consistent response envelope across all endpoints.
type APIResponse[T any] struct {
	Data  T        `json:"data,omitempty"`
	Meta  *APIMeta `json:"meta,omitempty"`
	Error any      `json:"error,omitempty"`
}

type APIMeta struct {
	Page       int   `json:"page,omitempty"`
	Limit      int   `json:"limit,omitempty"`
	TotalItems int64 `json:"total_items,omitempty"`
}

// WriteSuccessResponse serializes a success envelope.
func WriteSuccessResponse[T any](w http.ResponseWriter, statusCode int, data T, meta *APIMeta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(APIResponse[T]{
		Data: data,
		Meta: meta,
	})
}

// NormalizeEmail cleans and canonicalizes email addresses.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// UserAccount is internal database domain model with sensitive data.
type UserAccount struct {
	ID           string
	Email        string
	PasswordHash string // Sensitive
	SSN          string // Sensitive
	CreatedAt    time.Time
}

// UserAccountResponseDTO is filtered public DTO safe for API serialization.
type UserAccountResponseDTO struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// FilterUserOutput transforms domain model to filtered public DTO.
func FilterUserOutput(u *UserAccount) UserAccountResponseDTO {
	if u == nil {
		return UserAccountResponseDTO{}
	}
	return UserAccountResponseDTO{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}
```

### 2. Collection Handler Example

```go
func HandleListUsers(w http.ResponseWriter, r *http.Request) {
    accounts, totalCount := userRepo.ListUsers(r.Context(), 1, 20)

    // Filter sensitive fields into safe DTOs
    dtos := make([]UserAccountResponseDTO, len(accounts))
    for i, acc := range accounts {
        dtos[i] = FilterUserOutput(&acc)
    }

    meta := &APIMeta{
        Page:       1,
        Limit:      20,
        TotalItems: totalCount,
    }

    WriteSuccessResponse(w, http.StatusOK, dtos, meta)
}
```
