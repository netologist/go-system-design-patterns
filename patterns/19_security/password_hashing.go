package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrPasswordMismatch = errors.New("invalid password")
	ErrMalformedHash    = errors.New("malformed password hash string")
)

// PasswordHasher provides salted PBKDF2-style key derivation and constant-time verification.
type PasswordHasher struct {
	iterations int
	saltLength int
	keyLength  int
}

func NewPasswordHasher(iterations int) *PasswordHasher {
	if iterations <= 0 {
		iterations = 100000 // Recommended minimum
	}
	return &PasswordHasher{
		iterations: iterations,
		saltLength: 16,
		keyLength:  32,
	}
}

// HashPassword hashes plaintext password with a random cryptographically secure salt.
func (p *PasswordHasher) HashPassword(password string) (string, error) {
	salt := make([]byte, p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := p.deriveKey([]byte(password), salt, p.iterations, p.keyLength)

	// Format: $pbkdf2-sha256$iterations$saltHex$hashHex
	return fmt.Sprintf("$pbkdf2-sha256$%d$%s$%s",
		p.iterations, hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// VerifyPassword verifies plaintext password against encoded hash using constant-time comparison.
func (p *PasswordHasher) VerifyPassword(password, encodedHash string) error {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 5 || parts[1] != "pbkdf2-sha256" {
		return ErrMalformedHash
	}

	iterations, err := strconv.Atoi(parts[2])
	if err != nil {
		return ErrMalformedHash
	}

	salt, err := hex.DecodeString(parts[3])
	if err != nil {
		return ErrMalformedHash
	}

	expectedHash, err := hex.DecodeString(parts[4])
	if err != nil {
		return ErrMalformedHash
	}

	actualHash := p.deriveKey([]byte(password), salt, iterations, len(expectedHash))

	if subtle.ConstantTimeCompare(actualHash, expectedHash) != 1 {
		return ErrPasswordMismatch
	}

	return nil
}

func (p *PasswordHasher) deriveKey(password, salt []byte, iterations, keyLen int) []byte {
	// Standard PBKDF2 with SHA-256
	prf := func(key, msg []byte) []byte {
		h := sha256.New()
		h.Write(key)
		h.Write(msg)
		return h.Sum(nil)
	}

	out := make([]byte, keyLen)
	block := prf(password, salt)
	copy(out, block)

	for range iterations {
		block = prf(password, block)
		for j := 0; j < len(out) && j < len(block); j++ {
			out[j] ^= block[j]
		}
	}

	return out
}
