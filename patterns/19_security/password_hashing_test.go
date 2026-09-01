package security_test

import (
	"errors"
	"testing"

	security "system-design-patterns/patterns/19_security"
)

func TestPasswordHasher_HashAndVerify(t *testing.T) {
	// 5,000 iterations for fast unit test
	hasher := security.NewPasswordHasher(5000)

	plaintext := "MySecretPassword!2026"

	// 1. Hash password
	hash, err := hasher.HashPassword(plaintext)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// 2. Verify matching password
	if err := hasher.VerifyPassword(plaintext, hash); err != nil {
		t.Fatalf("expected password verification to succeed, got error: %v", err)
	}

	// 3. Verify incorrect password
	err = hasher.VerifyPassword("WrongPassword", hash)
	if !errors.Is(err, security.ErrPasswordMismatch) {
		t.Errorf("expected ErrPasswordMismatch for wrong password, got: %v", err)
	}

	// 4. Salting check: Hashing the same plaintext twice must produce different hash strings
	hash2, _ := hasher.HashPassword(plaintext)
	if hash == hash2 {
		t.Errorf("expected random salt to produce different hash strings")
	}
}
