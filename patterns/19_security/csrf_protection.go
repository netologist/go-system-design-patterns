package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrCSRFInvalid = errors.New("csrf validation failed: invalid or tampered token")
	ErrCSRFExpired = errors.New("csrf validation failed: token expired")
)

// CSRFManager creates and validates cryptographic double-submit tokens.
type CSRFManager struct {
	secret []byte
	ttl    time.Duration
}

func NewCSRFManager(secret string, ttl time.Duration) *CSRFManager {
	if ttl == 0 {
		ttl = 1 * time.Hour
	}
	return &CSRFManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}
// GenerateToken creates an HMAC token bound to a session and expiry timestamp.
func (m *CSRFManager) GenerateToken(sessionID string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	nonceHex := hex.EncodeToString(nonce)
	expiresAt := time.Now().Add(m.ttl).Unix()

	payload := fmt.Sprintf("%s:%s:%d", sessionID, nonceHex, expiresAt)
	sig := m.computeHMAC(payload)

	return fmt.Sprintf("%s.%s", payload, hex.EncodeToString(sig)), nil
}

// ValidateToken verifies token integrity, session binding, and expiration using constant-time comparison.
func (m *CSRFManager) ValidateToken(sessionID, token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ErrCSRFInvalid
	}

	payload, sigHex := parts[0], parts[1]
	providedSig, err := hex.DecodeString(sigHex)
	if err != nil {
		return ErrCSRFInvalid
	}

	expectedSig := m.computeHMAC(payload)
	if !hmac.Equal(expectedSig, providedSig) {
		return ErrCSRFInvalid
	}

	payloadParts := strings.Split(payload, ":")
	if len(payloadParts) != 3 {
		return ErrCSRFInvalid
	}

	tokenSessionID := payloadParts[0]
	expStr := payloadParts[2]

	if tokenSessionID != sessionID {
		return ErrCSRFInvalid
	}

	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() >= exp {
		return ErrCSRFExpired
	}

	return nil
}

func (m *CSRFManager) computeHMAC(data string) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}
