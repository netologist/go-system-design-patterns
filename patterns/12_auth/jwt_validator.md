# Cryptographic JWT Validation & Signing

## 1. Overview & Concept

JSON Web Tokens (JWT, RFC 7519) are the de facto standard for stateless, digitally signed identity assertions across distributed microservices. A JWT encodes claims (such as user identity, roles, and expiration timestamps) into a compact, URL-safe format composed of three dot-separated components:

$$\text{JWT} = \text{Base64Url}(\text{Header}) \mathbin{\Vert} \text{"."} \mathbin{\Vert} \text{Base64Url}(\text{Payload}) \mathbin{\Vert} \text{"."} \mathbin{\Vert} \text{Base64Url}(\text{Signature})$$

The **JWT Validator** pattern provides a hardened, cryptographically secure implementation for signing and verifying tokens using HMAC-SHA256 (`HS256`). It enforces constant-time signature verification, strict structural parsing, payload deserialization, and expiration validation.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without rigorous, hardened JWT parsing and verification:

* **Timing Side-Channel Attacks:** Using naive string or byte equality (`string(sig1) == string(sig2)`) allows remote attackers to measure microsecond latency differences in byte-by-byte comparisons to forge valid cryptographic signatures.
* **Algorithm Confusion / "none" Algorithm Exploits:** Insecure JWT parsers that blindly trust the `alg` header inside untrusted tokens can be tricked into accepting unsigned tokens (`"alg": "none"`) or asymmetric RSA tokens verified against symmetric public keys.
* **Expired Token Acceptance:** Failing to strictly validate `exp` timestamps allows stolen or retired access tokens to remain valid indefinitely.
* **Base64 Padding / URL Encoding Mismatches:** Standard Base64 uses `+`, `/`, and `=` padding, which breaks when transmitted in HTTP headers or query parameters without URL-safe encoding (`base64.RawURLEncoding`).

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

The validation flow inspects token structure, computes the HMAC digest, and validates temporal claims before deserializing identity.

```
                      Raw Token: "header.payload.signature"
                                     │
                                     ▼
                      ┌────────────────────────────┐
                      │  strings.Split(token, ".") │
                      └──────────────┬─────────────┘
                                     │
                       ┌─────────────┴─────────────┐
                       │                           │
                [len(parts) != 3]           [len(parts) == 3]
                       │                           │
                       ▼                           ▼
              ErrInvalidTokenFormat         Compute Expected HMAC
                                            hmac(secret, "header.payload")
                                                   │
                                                   ▼
                                      ┌─────────────────────────┐
                                      │ hmac.Equal(exp, prov)   │ ◄── Constant-Time Check
                                      └────────────┬────────────┘
                                                   │
                                     ┌─────────────┴─────────────┐
                                     │                           │
                                [Mismatch]                    [Match]
                                     │                           │
                                     ▼                           ▼
                            ErrInvalidSignature        Base64 Decode Payload
                                                                 │
                                                                 ▼
                                                       Check ExpiresAt >= now
                                                                 │
                                                   ┌─────────────┴─────────────┐
                                                   │                           │
                                              [Expired]                    [Valid]
                                                   │                           │
                                                   ▼                           ▼
                                            ErrTokenExpired             Return JWTClaims
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Constant-Time Verification (`hmac.Equal`)
Signature verification MUST use `hmac.Equal(expectedSig, providedSig)`. `hmac.Equal` executes in constant time regardless of where the first byte mismatch occurs, eliminating timing side-channel vulnerabilities.

### 2. Secret Key Entropy
For HMAC-SHA256 (`HS256`), the shared secret must possess at least 256 bits (32 bytes) of cryptographically secure random entropy. Using weak or short passwords (e.g. `"secret123"`) allows offline brute-force cracking using tools like Hashcat.

### 3. Payload Confidentiality vs. Integrity
JWT signatures guarantee **integrity** and **authenticity**, but NOT **confidentiality**. Claims inside standard JWTs are merely Base64Url-encoded JSON, readable by anyone inspecting network traffic. Never store sensitive secrets (passwords, private keys, unmasked PII) inside JWT claims.

### 4. Stateless Revocation Challenges
Because JWT verification is stateless, revoking an individual leaked token prior to its expiration (`exp`) requires maintaining a distributed blocklist (e.g. in Redis) or combining short-lived JWTs (5–15 minutes) with rotating refresh tokens.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/12_auth/jwt_validator.go`)

```go
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
```

### 2. HTTP Authentication Middleware Integration

```go
func JWTAuthMiddleware(validator *auth.JWTValidator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if !strings.HasPrefix(authHeader, "Bearer ") {
                http.Error(w, `{"error":"missing_bearer_token"}`, http.StatusUnauthorized)
                return
            }

            rawToken := strings.TrimPrefix(authHeader, "Bearer ")
            claims, err := validator.ValidateToken(rawToken)
            if err != nil {
                http.Error(w, `{"error":"invalid_or_expired_token"}`, http.StatusUnauthorized)
                return
            }

            user := &auth.AuthenticatedUser{
                ID:    claims.Subject,
                Email: claims.Email,
                Roles: claims.Roles,
            }

            ctx := auth.WithUser(r.Context(), user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```
