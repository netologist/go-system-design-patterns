package auth_test

import (
	"errors"
	"testing"
	"time"

	auth "system-design-patterns/patterns/12_auth"
)

func TestJWTValidator_SignAndValidate(t *testing.T) {
	secret := "my-super-secret-signing-key-32bytes"
	validator := auth.NewJWTValidator(secret)

	now := time.Now()
	claims := auth.JWTClaims{
		Subject:   "usr-123",
		Email:     "hasan@example.com",
		Roles:     []string{"admin", "engineer"},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(1 * time.Hour).Unix(),
	}

	token, err := validator.SignToken(claims)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	validated, err := validator.ValidateToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if validated.Subject != "usr-123" || validated.Email != "hasan@example.com" {
		t.Errorf("claims mismatch: %+v", validated)
	}
}

func TestJWTValidator_ExpiredToken(t *testing.T) {
	validator := auth.NewJWTValidator("secret")

	// Expired 10 minutes ago
	claims := auth.JWTClaims{
		Subject:   "usr-123",
		ExpiresAt: time.Now().Add(-10 * time.Minute).Unix(),
	}

	token, _ := validator.SignToken(claims)
	_, err := validator.ValidateToken(token)

	if !errors.Is(err, auth.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestJWTValidator_TamperedSignature(t *testing.T) {
	validator := auth.NewJWTValidator("secret")
	validatorAttacker := auth.NewJWTValidator("wrong-secret")

	claims := auth.JWTClaims{
		Subject:   "usr-attacker",
		ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
	}

	forgedToken, _ := validatorAttacker.SignToken(claims)

	// Validate using genuine server secret
	_, err := validator.ValidateToken(forgedToken)
	if !errors.Is(err, auth.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature for forged token, got: %v", err)
	}
}
