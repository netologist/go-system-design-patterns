# Cross-Site Request Forgery (CSRF) Protection Pattern

## 1. Overview & Concept
The **Cross-Site Request Forgery (CSRF) Protection Pattern** prevents unauthorized state-changing commands from being executed on behalf of an authenticated user by a malicious third-party website. In cookie-based session architectures, web browsers automatically attach stored session cookies (including session IDs and authentication tokens) to cross-origin HTTP requests (such as `POST` forms, fetch calls, or embedded image/script tags) made against the target domain.

To prevent this attack vector, this pattern implements **Cryptographically Signed Double-Submit CSRF Tokens**:
1. **Cryptographic Token Generation:** Generating a cryptographically random nonce bound to the user's `sessionID` and an expiration timestamp, authenticated with an HMAC-SHA256 signature using a server secret.
2. **Double-Submit Verification:** The client receives the token (e.g., in a response header or safe cookie) and must submit it in a custom request header (`X-CSRF-Token`) or hidden form field for state-changing operations (`POST`, `PUT`, `DELETE`, `PATCH`).
3. **Constant-Time HMAC Validation:** The server re-computes and verifies the signature using constant-time comparison (`hmac.Equal`), ensuring tokens cannot be forged or modified across origins.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
CSRF vulnerabilities (CWE-352) allow malicious websites visited by an authenticated user to perform unwanted actions without the user's knowledge or consent.

### Failure Scenarios Without This Pattern
- **Unauthorized Financial Transactions:** A user visits a malicious forum containing `<form action="https://bank.com/transfer" method="POST"><input name="to" value="attacker"/><input name="amount" value="5000"/></form><script>form.submit()</script>`. The browser attaches the bank's session cookie and transfers money automatically.
- **Account Takeover & Password Reset:** Malicious cross-origin scripts submit requests to `POST /api/user/email` to change the account recovery email address to the attacker's email.
- **State-Changing API Abuse:** Internal administrative portals lacking CSRF protection can be tricked into disabling firewall rules or granting admin privileges when an administrator browses external web pages.
- **Inadequate Protection by `SameSite` Cookies Alone:** While `SameSite=Lax` mitigates standard GET navigations, top-level POST form submissions or older browsers with incomplete `SameSite` support remain vulnerable.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The `CSRFManager` issues and validates stateless HMAC tokens bound to the active session and timestamp:

```
 [ Client Session Active: SessionID = "sess_abc123" ]
                        │
                        ▼
         ┌──────────────────────────────┐
         │  CSRFManager.GenerateToken   │
         │  - 16-byte random nonce      │
         │  - Expiry timestamp          │
         │  - HMAC-SHA256(secret)       │
         └──────────────┬───────────────┘
                        │
                        ▼
         [ Token: "sess_abc123:nonce:exp.signature" ]
                        │
         Sent to Client via X-CSRF-Token Header
                        │
 ───────────────────────┼───────────────────────────────────
                        │
  [ Incoming State-Changing Request: POST /api/transfer ]
  Headers:
    Cookie: session_id=sess_abc123
    X-CSRF-Token: sess_abc123:nonce:exp.signature
                        │
                        ▼
         ┌──────────────────────────────┐
         │  CSRFManager.ValidateToken   │
         └──────────────┬───────────────┘
                        │
       1. Split token into payload and sigHex
       2. Re-compute HMAC over payload using secret
       3. Constant-time signature verification: hmac.Equal()
       4. Session Binding: token.sessionID == cookie.sessionID?
       5. Expiration Check: time.Now().Unix() < token.exp?
                        │
         ┌──────────────┴──────────────┐
       [Valid]                      [Invalid]
         │                             │
         ▼                             ▼
  [Pass to Handler]          [Return 403 Forbidden]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Use Constant-Time Comparison:** Always compare HMAC signatures using `hmac.Equal` or `subtle.ConstantTimeCompare` to eliminate timing side-channel attacks.
- **Bind Token to Session ID:** A CSRF token generated for user A must never be valid for user B. Cryptographically binding `sessionID` into the signed payload prevents token-fixation attacks.
- **Set Bounded Token Expiry:** Enforce a strict time-to-live (e.g., 1 hour) so leaked tokens become invalid quickly.
- **Combine with SameSite Cookie Attributes:** Always configure session cookies with `SameSite=Strict` or `SameSite=Lax`, `Secure=true`, and `HttpOnly=true` as defense-in-depth.
- **Exempt Safe Idempotent Methods Only:** Exempt `GET`, `HEAD`, `OPTIONS`, and `TRACE` requests from CSRF validation, provided they have zero side-effects.

### Pitfalls to Avoid
- **Storing Plaintext Tokens in Database:** Stateless HMAC tokens eliminate database lookups. Avoid centralized Redis/SQL token tables unless immediate per-token revocation is strictly required.
- **Validating CSRF on GET Requests:** Never allow state changes on GET endpoints (e.g., `/delete?id=5`), as GET requests are easily triggered via `<img>` tags.
- **Reflecting Tokens in URLs:** Never pass CSRF tokens in query parameters, which leak in web server access logs, proxies, and HTTP `Referer` headers.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/19_security/csrf_protection.go`
```go
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
```

### Production CSRF Middleware Example
```go
func CSRFMiddleware(manager *security.CSRFManager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Allow safe read-only methods
            if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
                next.ServeHTTP(w, r)
                return
            }

            sessionCookie, err := r.Cookie("session_id")
            if err != nil || sessionCookie.Value == "" {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }

            csrfToken := r.Header.Get("X-CSRF-Token")
            if csrfToken == "" {
                http.Error(w, "missing csrf token", http.StatusForbidden)
                return
            }

            if err := manager.ValidateToken(sessionCookie.Value, csrfToken); err != nil {
                http.Error(w, "invalid or expired csrf token", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```
