# Partial Update (PATCH) with Pointer Semantics

## Overview & Definition

In REST APIs, `PUT` represents a complete replacement of a resource, whereas `PATCH` (RFC 5789) represents a partial modification of specified attributes.

In Go, differentiating between a field that was **omitted by the client** (which should remain unchanged) versus a field that was **explicitly sent with a zero value** (e.g. `""`, `0`, or `false`, which should be updated) is impossible when using standard value types.

The **Partial Update with Pointer Semantics** pattern solves this ambiguity by defining DTO struct fields as pointers (`*string`, `*int`, `*bool`):
* `field == nil`: Field was omitted by the client $\rightarrow$ Do not modify the existing database entity.
* `field != nil`: Field was explicitly provided $\rightarrow$ Validate and apply `*field` to the entity (even if it is empty string `""` or `0`).

---

## Problem Statement

Using plain value types for `PATCH` endpoints introduces severe data corruption bugs:

* **Accidental Zero-Value Overwrites:** If a client submits `{"bio": "Updated bio"}` without providing `display_name` or `age`, a Go struct with `DisplayName string` and `Age int` deserializes them as `""` and `0`. If applied to the database, the user's name is wiped out and age reset to 0.
* **Inability to Clear Fields:** A client cannot intentionally reset an integer counter to `0` or clear an optional string to `""` if zero-values are interpreted as "do not change".
* **Complex Custom Deserialization:** Writing custom map parsers (`map[string]any`) loses compile-time type safety, JSON tag mappings, and swagger generation.

---

## Architectural Mechanism & Flow

```
                      Incoming HTTP PATCH /users/123
                      {"display_name": "Alice"}  <-- Note: bio & age omitted
                                   |
                                   v
                      +------------------------------------------+
                      | PatchUserProfileRequest DTO:             |
                      |   DisplayName: *("Alice")                |
                      |   Bio:         nil (omitted)             |
                      |   Age:         nil (omitted)             |
                      +------------------------------------------+
                                   |
                                   v
                      +------------------------------------------+
                      |            patch.Validate()              |
                      +------------------------------------------+
                                   |
                                   v
                      +------------------------------------------+
                      |       patch.ApplyUpdates(profile)        |
                      +------------------------------------------+
                                   |
             +---------------------+---------------------+
             |                     |                     |
             v                     v                     v
   [DisplayName != nil]       [Bio == nil]          [Age == nil]
             |                     |                     |
    profile.DisplayName         [No-op]               [No-op]
      = *DisplayName        (Preserves existing)  (Preserves existing)
             |
             v
   [Save profile to DB]
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Separate DTOs from Domain Entities:** Keep `PatchRequest` DTOs (using pointers) strictly decoupled from core domain/database entities (`UserProfile`).
* **Validate Only Provided Fields:** In `patch.Validate()`, perform validations conditionally: `if p.Age != nil && *p.Age < 0 { return ErrInvalidAge }`.
* **Generate Dynamic SQL Updates:** In repositories, construct dynamic `UPDATE users SET display_name = $1 WHERE id = $2` queries that only update columns where pointers were non-nil, minimizing write-ahead log (WAL) and row-lock overhead.

### Common Pitfalls
* **Nil Pointer Dereferences:** Always check `p.Field != nil` before dereferencing `*p.Field` to prevent runtime panics.
* **Distinguishing `null` from Omitted:** If your API requires distinguishing between a field set explicitly to JSON `null` versus a field completely omitted from the JSON payload, use a tri-state wrapper library (e.g. `nullable.Type[T]`) or custom JSON unmarshaling.

---

## Code Walkthrough & Usage

### 1. Implementation (`partial_update.go`)

```go
package apidesign

import (
	"errors"
	"strings"
)

// UserProfile represents target domain entity.
type UserProfile struct {
	ID          string
	DisplayName string
	Bio         string
	Age         int
}

// PatchUserProfileRequest DTO uses pointers to represent optional partial update fields.
// nil = field omitted (do not change)
// non-nil = field present (apply update, even if empty string or 0)
type PatchUserProfileRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	Age         *int    `json:"age,omitempty"`
}

// Validate ensures partial update fields conform to domain rules.
func (p *PatchUserProfileRequest) Validate() error {
	if p.DisplayName != nil && strings.TrimSpace(*p.DisplayName) == "" {
		return errors.New("display_name cannot be empty if provided")
	}
	if p.Age != nil && *p.Age < 0 {
		return errors.New("age cannot be negative")
	}
	return nil
}

// ApplyUpdates modifies the entity only for fields explicitly provided in the patch request.
func (p *PatchUserProfileRequest) ApplyUpdates(profile *UserProfile) {
	if profile == nil {
		return
	}

	if p.DisplayName != nil {
		profile.DisplayName = *p.DisplayName
	}
	if p.Bio != nil {
		profile.Bio = *p.Bio
	}
	if p.Age != nil {
		profile.Age = *p.Age
	}
}
```

### 2. Handler Usage Example

```go
func HandlePatchUserProfile(w http.ResponseWriter, r *http.Request) {
    var patch PatchUserProfileRequest
    if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    if err := patch.Validate(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    profile, err := userRepo.GetByID(r.Context(), "usr-123")
    if err != nil {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }

    // Mutate only fields present in patch
    patch.ApplyUpdates(profile)

    if err := userRepo.Save(r.Context(), profile); err != nil {
        http.Error(w, "Database update failed", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```
