package security_test

import (
	"strings"
	"testing"

	security "system-design-patterns/patterns/19_security"
)

func TestValidateAndSanitizeUser_XSSSanitization(t *testing.T) {
	username := "john_doe"
	email := "john@example.com"
	maliciousBio := `<script>alert('XSS Attack!')</script>`

	cleanBio, err := security.ValidateAndSanitizeUser(username, email, maliciousBio)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if strings.Contains(cleanBio, "<script>") {
		t.Errorf("raw script tag present in sanitized bio: %s", cleanBio)
	}
	if !strings.Contains(cleanBio, "&lt;script&gt;") {
		t.Errorf("expected HTML escaped output, got: %s", cleanBio)
	}
}

func TestValidateAndSanitizeUser_InvalidInputs(t *testing.T) {
	// Invalid username with spaces / special characters
	_, err := security.ValidateAndSanitizeUser("user with space", "user@example.com", "bio")
	if err == nil {
		t.Error("expected error on username with spaces")
	}

	// Invalid email
	_, err = security.ValidateAndSanitizeUser("valid_user", "invalid_email_no_at", "bio")
	if err == nil {
		t.Error("expected error on invalid email")
	}
}
