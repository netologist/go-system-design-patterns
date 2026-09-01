# Secret Sanitizer Pattern

## 1. Overview & Concept
The **Secret Sanitizer** pattern prevents sensitive credentials (passwords, API tokens, cryptographic private keys, database connection strings) from being accidentally leaked into application logs, traces, error dumps, or JSON diagnostic outputs.

The pattern achieves this by providing:
1. **A Generic `Secret[T]` Type**: An unexported wrapper that encapsulates sensitive values and intercepts standard Go formatting interfaces (`fmt.Stringer`, `fmt.GoStringer`, `json.Marshaler`) to output `[REDACTED]` whenever the struct is printed or serialized.
2. **Explicit Extraction (`.Expose()`)**: Developers must explicitly call `.Expose()` to access the raw underlying credential, making credential access visible and auditable during code reviews.
3. **Map and URL Scrubbers**: Helper functions (`SanitizeMap`, `RedactURL`) that scrub sensitive dictionary keys and userinfo credentials from database URLs before logging.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Accidental credential exposure in application logs is one of the most critical security vulnerabilities (OWASP A02/A09):

- **Plaintext Passwords in Application Logs**: Passing a configuration struct directly to `log.Printf("%+v", cfg)` or `slog.Info("config", "cfg", cfg)` prints plaintext database passwords, AWS secret keys, and JWT signing secrets into centralized logging systems (Datadog, Elastic, Loki, CloudWatch).
- **Log Aggregator Data Breaches**: Once secrets land in plaintext logs, any internal developer or external third party with read access to log indices can access production databases and private infrastructure.
- **Leaked Tokens in Sentry & Stack Traces**: Panic recovery handlers often serialize the runtime environment and request context into error monitoring systems, exposing bearer tokens and database connection strings.
- **Accidental JSON Serialization**: Exporting config structs over debugging HTTP endpoints (e.g., `/debug/vars` or `/debug/config`) exposes unmasked credentials to unauthorized users.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Redaction & Interception Mechanism

```
Caller / Logger / JSON Serializer               Secret[T] Wrapper                     Underlying Value (string)
                |                                      |                                         |
                |--- fmt.Sprintf("%v", secret) ------->|                                         |
                |                                      |-- Intercepts fmt.Stringer               |
                |<-- Returns "[REDACTED]" -------------|                                         |
                |                                      |                                         |
                |--- json.Marshal(secret) ------------>|                                         |
                |                                      |-- Intercepts json.Marshaler             |
                |<-- Returns "\"[REDACTED]\"" --------|                                         |
                |                                      |                                         |
                |--- secret.Expose() (Explicit Call) ->|                                         |
                |                                      |--- Returns Raw Credential ------------->|
                |<-- Receives Plaintext Password ------|                                         |
```

### Architectural Component Structure (ASCII)

```
 +-------------------------------------------------------------------------+
 |                           SECRET SANITIZER                              |
 |                                                                         |
 |  Generic Container: Secret[T any]                                       |
 |  - value: T (Unexported field)                                          |
 |                                                                         |
 |  Interface Implementations:                                             |
 |  +-> String() string:            Returns "[REDACTED]" for %s, %v        |
 |  +-> GoString() string:          Returns "[REDACTED]" for %#v           |
 |  +-> MarshalJSON() ([]byte, err): Returns json("[REDACTED]")            |
 |  +-> Expose() T:                 Explicit gateway to raw plaintext      |
 |                                                                         |
 |  Sanitization Utilities:                                                |
 |  +-> SanitizeMap(map[string]string) -> Masks keys containing "pass",    |
 |      "secret", "token", "key", "auth", "credential", "private", "cert"  |
 |  +-> RedactURL(string) -> Replaces "user:pass@" with "user:****@"      |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Wrap All Credentials in `Secret[T]`**: Store passwords, tokens, and private keys as `Secret[string]` inside configuration structs.
- **Redact URLs in Error Messages**: Always pass database connection errors or remote URLs through `RedactURL()` before wrapping them in returning errors.
- **Audit `Expose()` Usage**: Use linters or code review checklists to ensure `.Expose()` is only called when initializing network drivers, never inside logging middleware.
- **Combine with Structured Logging**: Integrate `Secret[T]` with structured loggers (`slog.LogValuer`) to guarantee masking across all logging handlers.

### Common Pitfalls & Anti-Patterns
- **Accidental Type Conversions**: Casting `string(secret.Expose())` and storing it in a temporary variable that gets logged later in the call stack.
- **Relying Solely on Keyword Filtering**: Assuming `SanitizeMap` will catch every secret; custom credential names (e.g., `MY_CUSTOM_PIN`) can evade generic keyword filters if not wrapped in `Secret[T]`.
- **Exposing Secrets in URL Queries**: Forgetting that query parameters (e.g., `?api_key=xyz`) in URLs require sanitization just like userinfo credentials.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/02_config/secret_sanitizer.go`.

### Core Types & Signatures

```go
package config

const RedactedPlaceholder = "[REDACTED]"

type Secret[T any] struct {
	// unexported field: value
}

func NewSecret[T any](val T) Secret[T]
func (s Secret[T]) Expose() T
func (s Secret[T]) String() string
func (s Secret[T]) GoString() string
func (s Secret[T]) MarshalJSON() ([]byte, error)

func SanitizeMap(input map[string]string) map[string]string
func RedactURL(rawURL string) string
```

### Complete End-to-End Example

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"patterns/02_config"
)

type AppConfig struct {
	DBUser     string
	DBPassword config.Secret[string]
	APIKey     config.Secret[string]
}

func main() {
	cfg := AppConfig{
		DBUser:     "admin",
		DBPassword: config.NewSecret("super_secret_db_password_123"),
		APIKey:     config.NewSecret("sk_live_9837498172948712"),
	}

	// 1. Safe printing with fmt - outputs [REDACTED]
	fmt.Printf("Standard print: %+v\n", cfg)
	fmt.Printf("Go syntax print: %#v\n", cfg)

	// 2. Safe JSON serialization - secrets are masked
	jsonData, _ := json.Marshal(cfg)
	fmt.Printf("JSON serialization: %s\n", string(jsonData))

	// 3. Explicit exposure only when authenticating
	log.Printf("Connecting to DB with user %s and secret len %d",
		cfg.DBUser, len(cfg.DBPassword.Expose()))

	// 4. URL redaction
	rawURL := "postgres://postgres:myPassword@localhost:5432/mydb?sslmode=disable"
	fmt.Printf("Sanitized URL: %s\n", config.RedactURL(rawURL))
}
```
