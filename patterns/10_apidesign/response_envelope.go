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
