# Refresh Token Rotation & Breach Detection (Token Families)

## 1. Overview & Concept

Long-lived sessions in modern web and mobile applications require balancing user experience (avoiding frequent re-logins) with security (limiting the window of exposure if a credential is leaked). Standard industry practice issues short-lived **Access Tokens** (5–15 minutes) paired with longer-lived **Refresh Tokens** (7–30 days).

However, static refresh tokens present severe security vulnerabilities if intercepted. The **Refresh Token Rotation (RTR)** pattern (mandated by OAuth 2.0 Security BCP and OAuth 2.1) ensures that **every time a refresh token is used, it is invalidated and replaced with a newly issued refresh token**.

Furthermore, by organizing tokens into a cryptographically linked **Token Family**, the system implements **Automatic Breach Detection**: if an already-used (historical) refresh token is presented again (a sign of a token replay attack or stolen credential), the authorization server instantly revokes the entire token family, terminating all sessions for that device.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without refresh token rotation and token family breach tracking:

* **Indefinite Token Exfiltration / Persistent Compromise:** If an attacker steals a static refresh token from local storage or an insecure network transport, they can generate new access tokens indefinitely without the legitimate user or server ever knowing.
* **Inability to Detect Replay Attacks:** In standard systems without historical token tracking, presenting an old token either fails silently or grants access, giving no telemetry on whether a credential theft event took place.
* **Session Hijacking Across Devices:** Without granular token family boundaries per device/client, revoking a compromised token requires wiping the entire user record or revoking every active session across all devices.
* **Frontend Concurrent Tab Races:** If a user opens 5 browser tabs simultaneously and all 5 tabs attempt to rotate the refresh token at the same millisecond, strict rotation without a brief grace period can accidentally trigger false-positive breach revocations.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

Every login creates an isolated `TokenFamily`. Rotating a token advances the active pointer and archives the previous token in `UsedTokens`.

```
               [User Login] ──► CreateFamily()
                                     │
                                     ▼
                     ┌───────────────────────────────┐
                     │         TokenFamily           │
                     │  FamilyID: "fam-1"            │
                     │  CurrentToken: "Token-1"      │
                     │  UsedTokens: {}               │
                     │  Revoked: false               │
                     └───────────────┬───────────────┘
                                     │
                 Legitimate Rotation (Token-1 -> Token-2)
                                     │
                                     ▼
                     ┌───────────────────────────────┐
                     │         TokenFamily           │
                     │  FamilyID: "fam-1"            │
                     │  CurrentToken: "Token-2"      │
                     │  UsedTokens: {"Token-1": t1}  │
                     │  Revoked: false               │
                     └───────────────┬───────────────┘
                                     │
                                     ▼
                  Attacker Replays Intercepted "Token-1"
                                     │
                                     ▼
                     ┌───────────────────────────────┐
                     │   Check fam.UsedTokens[Token-1]│
                     │   Match Found! (BREACH DETECTED)│
                     └───────────────┬───────────────┘
                                     │
                                     ▼
                     ┌───────────────────────────────┐
                     │       EMERGENCY REVOCATION    │
                     │  fam.Revoked = true           │
                     │  Return ErrTokenReused        │
                     │  Reject ALL subsequent tokens │
                     └───────────────────────────────┘
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### 1. Replay Attack Revocation Rule
When `Rotate(familyID, presentedToken)` is invoked:
1. If `familyID` is revoked or not found: Return `ErrTokenInvalidOrRevoked`.
2. If `presentedToken` exists in `UsedTokens`: An attacker or compromised client is replaying an obsolete token. **Instantly revoke the entire family** (`fam.Revoked = true`) and return `ErrTokenReusedTheftDetected`.
3. If `presentedToken == fam.CurrentToken`: Archive current token into `UsedTokens`, generate a new secure 32-byte cryptographic token, and set `fam.CurrentToken = newToken`.

### 2. Distributed Storage & Concurrency
In multi-node production clusters, token families must be persisted in a shared transactional datastore (Redis with Lua scripts or PostgreSQL with `SELECT ... FOR UPDATE`) to prevent race conditions during concurrent rotation attempts.

### 3. Grace Periods for Network Retries
In flaky mobile network environments, a client might send a refresh request, receive the new token, but disconnect before saving it locally. If the client retries with the previous token within a short window (e.g. 5–10 seconds), some production architectures allow returning the already-generated `CurrentToken` rather than triggering immediate breach revocation.

### 4. Pruning Expired Families
Used token maps and retired families must have a Time-To-Live (TTL) matching the absolute maximum session lifespan (e.g. 30 days) to prevent unbounded memory growth.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### 1. Core Implementation (`patterns/12_auth/token_rotation.go`)

```go
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
```

### 2. HTTP Token Refresh Handler Example

```go
type RefreshRequest struct {
    FamilyID     string `json:"family_id"`
    RefreshToken string `json:"refresh_token"`
}

func RefreshHandler(mgr *auth.TokenRotationManager, jwtVal *auth.JWTValidator) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req RefreshRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
            return
        }

        newRefreshToken, err := mgr.Rotate(req.FamilyID, req.RefreshToken)
        if err != nil {
            if errors.Is(err, auth.ErrTokenReusedTheftDetected) {
                // Log critical security breach event
                slog.Error("SECURITY BREACH: Refresh token replay attack detected", "family_id", req.FamilyID)
                http.Error(w, `{"error":"security_breach_session_revoked"}`, http.StatusUnauthorized)
                return
            }
            http.Error(w, `{"error":"invalid_or_revoked_token"}`, http.StatusUnauthorized)
            return
        }

        // Issue new 15-minute access token alongside rotated refresh token
        newAccessToken, _ := jwtVal.SignToken(auth.JWTClaims{
            Subject:   "user-100",
            ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
        })

        json.NewEncoder(w).Encode(map[string]string{
            "access_token":  newAccessToken,
            "refresh_token": newRefreshToken,
        })
    }
}
```
