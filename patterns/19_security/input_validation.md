# Input Validation & Sanitization Pattern

## 1. Overview & Concept
The **Input Validation & Sanitization Pattern** is a fundamental defense-in-depth security boundary that enforces strict structural, syntactic, and semantic constraints on all external data before it enters application business logic or persistence layers. In modern distributed systems and microservices, untrusted input arrives continuously through HTTP request bodies, URL query parameters, headers, message queues, and external webhooks.

This pattern operates on two complementary principles:
1. **Defensive Input Validation (Allowlisting / Strict Typing):** Verifying that incoming fields conform to rigorous formatting constraints, length limits, character sets, and business domains. Unknown or non-conforming characters are rejected outright.
2. **Context-Aware Sanitization & Output Encoding:** Neutralizing dangerous characters (such as HTML control characters `<>&"'`) to prevent Cross-Site Scripting (XSS) and injection attacks when content is rendered in browsers or forwarded to downstream services.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)
Without centralized and authoritative input validation at service entry points, systems suffer from critical security vulnerabilities and operational instability:

### Failure Scenarios Without This Pattern
- **Cross-Site Scripting (Stored & Reflected XSS):** Unsanitized user inputs containing script tags or malicious HTML payloads (e.g., `<script>alert(1)</script>` or `<img src=x onerror=stealCookies()>`) are persisted in the database and executed in victim browsers, leading to session hijacking and credential theft.
- **Resource Exhaustion & Denial of Service (DoS):** Unbounded string fields (e.g., bio fields containing megabytes of data) cause excessive memory allocations, GC pressure, network saturation, and database buffer exhaustion.
- **Data Integrity Corruption:** Unvalidated email addresses or usernames pollute the database with malformed data, causing downstream payment gateways, notification workers, and CRM integrations to crash or fail silently.
- **Malformed Internal State & Inconsistent Validation:** Validation rules scattered ad-hoc across controllers lead to bypasses when new endpoints, gRPC handlers, or background jobs process the same underlying data models without identical checks.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)
The validation mechanism acts as an authoritative entry gatekeeper. It evaluates inputs sequentially: validating structural identity (usernames, identifiers), syntax patterns (RFC-compliant emails), bounded length constraints, and applying canonical sanitization before passing clean data to domain services.

```
       [ Untrusted Client Request ]
                    │
                    ▼
     ┌─────────────────────────────┐
     │   Boundary Validation Gate  │
     └──────────────┬──────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
 1. Username Regex       2. Email Regex
 [3-30 Alphanumeric]     [Valid Domain & Format]
        │                       │
        └───────────┬───────────┘
                    ▼
          3. Payload Length Check
             [Bio <= 500 chars]
                    │
                    ▼
          4. Output Sanitization
        [html.EscapeString & Trim]
                    │
         ┌──────────┴──────────┐
      [Invalid]             [Valid]
         │                     │
         ▼                     ▼
 [Return 400 Bad Req]   [Pass to Business Logic]
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Best Practices & Hardening
- **Prefer Allowlisting Over Blocklisting:** Define explicit allowed character sets (e.g., `^[a-zA-Z0-9_-]+$`) rather than attempting to filter out known bad words or characters, which attackers routinely bypass using encoding tricks.
- **Precompile Regular Expressions:** Always compile `regexp.MustCompile` at package initialization (`var`) rather than compiling regex inside request handlers, avoiding severe CPU overhead and lock contention per request.
- **Enforce Strict Length Limits:** Bound every string, array, and nested object. Unbounded fields invite memory starvation and ReDoS attacks.
- **Validate at System Boundaries:** Run validation in the transport/adapter layer (HTTP handlers, gRPC interceptors) before calling domain services or persisting data.
- **Sanitize for Target Context:** Understand the sink context. HTML escaping is required for web rendering; SQL parameterization is required for databases; shell escaping is required for OS executions.

### Pitfalls to Avoid
- **Validating Post-Processing:** Validating strings after applying unescaping or transformations can introduce bypass vulnerabilities.
- **Inconsistent Client-Side Only Validation:** Client-side validation improves UX but provides zero security. Server-side validation must be strictly authoritative.
- **Over-Permissive Regular Expressions & ReDoS:** Naive regex patterns with nested quantifiers (e.g., `(a+)+$`) can cause catastrophic backtracking, freezing the Go runtime. Keep expressions simple and linear.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

### Core Implementation in `patterns/19_security/input_validation.go`
```go
package security

import (
	"errors"
	"html"
	"regexp"
	"strings"
)

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)
)

// ValidateAndSanitizeUser performs defensive input validation and XSS HTML sanitization.
func ValidateAndSanitizeUser(username, email, bio string) (cleanBio string, err error) {
	// 1. Username constraints
	if !usernameRegex.MatchString(username) {
		return "", errors.New("username must be 3-30 alphanumeric characters, underscores, or hyphens")
	}

	// 2. Email format constraints
	if !emailRegex.MatchString(email) {
		return "", errors.New("invalid email address format")
	}

	// 3. Bio length bounded check
	if len(bio) > 500 {
		return "", errors.New("bio exceeds maximum 500 characters limit")
	}

	// 4. Output Encoding / XSS Sanitization
	sanitizedBio := html.EscapeString(strings.TrimSpace(bio))

	return sanitizedBio, nil
}
```

### Production API Handler Example
```go
func HandleCreateUserProfile(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Username string `json:"username"`
        Email    string `json:"email"`
        Bio      string `json:"bio"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid json payload", http.StatusBadRequest)
        return
    }

    cleanBio, err := security.ValidateAndSanitizeUser(req.Username, req.Email, req.Bio)
    if err != nil {
        http.Error(w, err.Error(), http.StatusUnprocessableEntity)
        return
    }

    // Pass validated & clean data to domain service
    user := &domain.User{
        Username: req.Username,
        Email:    strings.ToLower(req.Email),
        Bio:      cleanBio,
    }
    _ = userService.Save(r.Context(), user)
}
```
