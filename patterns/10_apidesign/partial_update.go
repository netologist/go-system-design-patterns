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
