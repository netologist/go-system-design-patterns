package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidTokenFormat = errors.New("invalid jwt token format")
	ErrInvalidSignature   = errors.New("invalid jwt signature")
	ErrTokenExpired        = errors.New("jwt token is expired")
)

// JWTClaims represents standard and custom payload claims.
type JWTClaims struct {
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
}

// JWTValidator parses and verifies HMAC-SHA256 signed JWTs.
type JWTValidator struct {
	secret []byte
}

func NewJWTValidator(secret string) *JWTValidator {
	return &JWTValidator{secret: []byte(secret)}
}

// SignToken generates a valid HMAC-SHA256 JWT string.
func (v *JWTValidator) SignToken(claims JWTClaims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerBytes, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerBytes)

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := headerB64 + "." + payloadB64
	sig := v.computeHMAC(signingInput)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}

// ValidateToken verifies token format, signature, and expiration.
func (v *JWTValidator) ValidateToken(tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidTokenFormat
	}

	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]
	signingInput := headerB64 + "." + payloadB64

	expectedSig := v.computeHMAC(signingInput)
	providedSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil || !hmac.Equal(expectedSig, providedSig) {
		return nil, ErrInvalidSignature
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("malformed payload base64: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	// Verify expiration
	now := time.Now().Unix()
	if claims.ExpiresAt > 0 && now >= claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func (v *JWTValidator) computeHMAC(data string) []byte {
	h := hmac.New(sha256.New, v.secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}
