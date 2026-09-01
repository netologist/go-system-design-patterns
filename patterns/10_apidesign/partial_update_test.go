package apidesign_test

import (
	"testing"

	apidesign "system-design-patterns/patterns/10_apidesign"
)

func TestPatchUserProfile_PartialUpdates(t *testing.T) {
	profile := &apidesign.UserProfile{
		ID:          "usr-1",
		DisplayName: "Old Name",
		Bio:         "Software Engineer in London",
		Age:         30,
	}

	newName := "New Name"
	patch := apidesign.PatchUserProfileRequest{
		DisplayName: &newName, // Only updating DisplayName
		Bio:         nil,      // Omitted
		Age:         nil,      // Omitted
	}

	if err := patch.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	patch.ApplyUpdates(profile)

	if profile.DisplayName != "New Name" {
		t.Errorf("expected DisplayName updated to 'New Name', got: %s", profile.DisplayName)
	}
	if profile.Bio != "Software Engineer in London" {
		t.Errorf("Bio should not have been modified, got: %s", profile.Bio)
	}
	if profile.Age != 30 {
		t.Errorf("Age should not have been modified, got: %d", profile.Age)
	}
}
