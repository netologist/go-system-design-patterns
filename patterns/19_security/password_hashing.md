# Password Hashing & Key Derivation Pattern

## 1. Overview & Concept
The **Password Hashing & Key Derivation Pattern** provides cryptographically secure storage and verification of user authentication credentials. Plaintext passwords must never be stored in databases, transmitted in logs, or hashed using fast, unsalted hashing algorithms (such as MD5, SHA-1, or plain SHA-256), which are trivially reversed using precomputed rainbow tables or brute-forced at billions of hashes per second using GPUs and ASICs.

This pattern leverages **Salted Key Derivation (PBKDF2/Argon2/bcrypt)**:
1. **Cryptographically Random Unique Salt:** Generating a unique 16-byte cryptographically secure salt (`crypto/rand`) for each password hash, neutralizing rainbow table lookups.
2. **Tunable Work Factor (Computational Stretching):** Executing thousands of iterative hashing rounds (e.g., 100,000+ iterations) to increase the CPU/GPU cost for attackers.
3. **Self-Describing Encoded Hash Format:** Storing algorithm metadata, iteration count, salt, and hash together (e.g., `$pbkdf2-sha256$100000$saltHex$hashHex`), allowing work factors to be upgraded over time.
4. **Constant-Time Verification:** Employing `crypto/subtle.ConstantTimeCompare` during password verification to eliminate timing side-channel attacks.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
Credential leaks are catastrophic events. Without adaptive, salted key derivation algorithms, compromised database dumps result in mass user account compromise across the internet.

### Failure Scenarios Without This Pattern
- **Rainbow Table & Dictionary Reversal:** Plain MD5/SHA256 hashes allow attackers with precomputed tables to crack 90%+ of user passwords within minutes of a database breach.
- **Identical Password Discovery:** Without unique per-user salts, two users with the same password (e.g., `Password123!`) yield identical hash strings in the database, revealing duplicate credentials across user accounts.
- **High-Speed GPU Brute Forcing:** Fast cryptographic hash functions (SHA-256) compute billions of hashes per second per GPU. Salted slow KDFs (PBKDF2, Argon2id, bcrypt) force substantial latency and memory requirements per attempt.
- **Timing Side-Channel Attacks:** Using naive string comparison (`actualHash == expectedHash`) leaks information about how many leading bytes matched based on response time, allowing attackers to reconstruct valid hashes.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The password hashing and verification workflows enforce one-way derivation and constant-time equality checks:

```
[ User Registration / Password Change ]
                  │
                  ▼
   ┌──────────────────────────────┐
   │  PasswordHasher.HashPassword │
   └──────────────┬───────────────┘
                  │
   1. Generate 16-byte Salt: crypto/rand.Read(salt)
   2. Derive Key: PBKDF2-HMAC-SHA256(password, salt, iterations=100000, keyLen=32)
   3. Encode Self-Describing String:
      "$pbkdf2-sha256$<iterations>$<saltHex>$<hashHex>"
                  │
                  ▼
      [ Persist to Database ]

─────────────────────────────────────────────────────────────

[ User Login / Authentication ]
                  │
                  ▼
   ┌────────────────────────────────┐
   │ PasswordHasher.VerifyPassword  │
   └──────────────┬─────────────────┘
                  │
   1. Parse encoded hash string to extract:
      algorithm, iterations, salt, expectedHash
   2. Re-derive actualHash from candidate password using extracted salt and iterations
   3. Constant-Time Compare:
      subtle.ConstantTimeCompare(actualHash, expectedHash) == 1?
                  │
         ┌────────┴────────┐
       [Yes]              [No]
         │                 │
         ▼                 ▼
   [Login Success]   [ErrPasswordMismatch]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Use Constant-Time Comparison Always:** Use `subtle.ConstantTimeCompare` rather than standard string equality (`==`) or `bytes.Equal` to prevent byte-by-byte timing attacks.
- **Tune Work Factor to Server Hardware:** Balance security against API latency. A target duration of 50ms to 200ms per hash on server hardware is standard. Too fast allows attackers to brute-force; too slow enables CPU-exhaustion DoS attacks on login endpoints.
- **Store Algorithm Identifiers for Seamless Migration:** By embedding algorithm and iteration metadata in the hash string, systems can seamlessly upgrade from PBKDF2/bcrypt to Argon2id upon subsequent successful logins.
- **Apply Rate Limiting on Login Endpoints:** Combine password hashing with IP-based and username-based rate limiters to throttle brute-force attacks at the application gateway.

### Pitfalls to Avoid
- **Re-using Global or Static Salts:** A static salt across the entire database allows rainbow tables tailored to that specific salt. Always generate a fresh salt per password.
- **Truncating Passwords:** Legacy bcrypt truncates inputs at 72 bytes. Pre-hashing with SHA-256 or using PBKDF2/Argon2 avoids password truncation issues.
- **Hashing on the Client Side Alone:** Client-side hashing does not protect against replay attacks (the client-side hash simply becomes the password). Hashing must always be executed server-side.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/19_security/password_hashing.go`
```go
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
```

### Production Authentication Service Example
```go
type AuthService struct {
    hasher *security.PasswordHasher
    db     *sql.DB
}

func (s *AuthService) RegisterUser(ctx context.Context, email, plaintextPassword string) error {
    hash, err := s.hasher.HashPassword(plaintextPassword)
    if err != nil {
        return fmt.Errorf("failed to hash password: %w", err)
    }

    _, err = s.db.ExecContext(ctx, 
        "INSERT INTO users (email, password_hash, created_at) VALUES ($1, $2, NOW())", 
        email, hash)
    return err
}

func (s *AuthService) Authenticate(ctx context.Context, email, candidatePassword string) (bool, error) {
    var storedHash string
    err := s.db.QueryRowContext(ctx, 
        "SELECT password_hash FROM users WHERE email = $1", 
        email).Scan(&storedHash)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // Perform dummy verification to prevent timing attack on user existence
            _ = s.hasher.VerifyPassword("dummy", "$pbkdf2-sha256$100000$0000000000000000$00000000000000000000000000000000")
            return false, nil
        }
        return false, err
    }

    if err := s.hasher.VerifyPassword(candidatePassword, storedHash); err != nil {
        return false, nil // Invalid password
    }

    return true, nil // Authenticated successfully
}
```
