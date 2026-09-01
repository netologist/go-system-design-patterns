package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrTokenReusedTheftDetected = errors.New("refresh token reuse detected: token family revoked for security")
	ErrTokenInvalidOrRevoked    = errors.New("refresh token is invalid or family has been revoked")
)

func generateSecureToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// TokenFamily tracks a chain of rotated refresh tokens for a user session.
type TokenFamily struct {
	FamilyID     string
	UserID       string
	CurrentToken string
	UsedTokens   map[string]time.Time
	Revoked      bool
	CreatedAt    time.Time
}

// TokenRotationManager coordinates refresh token rotation and breach detection.
type TokenRotationManager struct {
	mu       sync.Mutex
	families map[string]*TokenFamily
}

func NewTokenRotationManager() *TokenRotationManager {
	return &TokenRotationManager{
		families: make(map[string]*TokenFamily),
	}
}

// CreateFamily initiates a new session family and issues the first refresh token.
func (m *TokenRotationManager) CreateFamily(userID string) (familyID string, initialToken string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fID := generateSecureToken()[:16]
	token := generateSecureToken()

	m.families[fID] = &TokenFamily{
		FamilyID:     fID,
		UserID:       userID,
		CurrentToken: token,
		UsedTokens:   make(map[string]time.Time),
		CreatedAt:    time.Now(),
	}

	return fID, token
}

// Rotate exchanges an existing refresh token for a new one.
// If an already-used token is presented (replay attack), the entire family is instantly revoked.
func (m *TokenRotationManager) Rotate(familyID, presentedToken string) (newToken string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fam, ok := m.families[familyID]
	if !ok || fam.Revoked {
		return "", ErrTokenInvalidOrRevoked
	}

	// 1. Detect Reuse / Replay Attack (Theft)
	if _, alreadyUsed := fam.UsedTokens[presentedToken]; alreadyUsed {
		// Compromise detected: Revoke entire token family
		fam.Revoked = true
		return "", ErrTokenReusedTheftDetected
	}

	// 2. Verify current token
	if fam.CurrentToken != presentedToken {
		return "", ErrTokenInvalidOrRevoked
	}

	// 3. Rotate to new token
	fam.UsedTokens[presentedToken] = time.Now()
	newToken = generateSecureToken()
	fam.CurrentToken = newToken

	return newToken, nil
}
