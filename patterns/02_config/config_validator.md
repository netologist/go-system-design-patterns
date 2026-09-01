# Config Validator Pattern

## 1. Overview & Concept
The **Config Validator** pattern provides a composable, declarative validation engine to verify application configuration against strict operational and domain constraints.

Instead of writing repetitive, ad-hoc `if` checks across bootstrap code, the `ConfigValidator` encapsulates standard enterprise validation rules—such as string non-emptiness, valid TCP port ranges, positive time durations, allowed enum sets, and well-formed URL schemes. It records individual field violations and combines them into an aggregated diagnostic error before the application initiates network listeners.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Deploying configurations without strict semantic validation causes subtle and catastrophic production failures:

- **Corrupted Connection URLs**: An invalid database connection string (e.g., missing scheme `postgresql://` or missing port) causes drivers to hang or throw obscure low-level socket errors minutes after container startup.
- **Port Collision & Out-of-Bound Ports**: A misconfigured port number (e.g., negative, `0`, or `70000`) causes bind errors that abort the service abruptly.
- **Invalid Timeout Values**: Configuring a timeout as `0` or negative (e.g., `-5s`) can cause HTTP clients or database connection pools to immediately fail every transaction.
- **Illegal Enum / Environment Names**: Typographical errors in environment names (e.g., `APP_ENV="production"` instead of `"prod"`) can cause services to activate development-only bypasses or disable authentication.
- **Fragmented Error Messages**: Failing on the first invalid field causes operators to cycle through multiple redeployments to discover subsequent configuration errors.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Validator Workflow Sequence

```
Raw Configuration Struct               ConfigValidator                      Validation Rules
          |                                   |                                     |
          |--- Validate Port ---------------->|--- ValidatePort(1..65535) --------->|
          |--- Validate Database URL -------->|--- ValidateURL(schemes: postgres) ->|
          |--- Validate Read Timeout -------->|--- ValidatePositiveDuration (>0) -->|
          |--- Validate Environment --------->|--- ValidateEnum("dev","staging") -->|
          |                                   |                                     |
          |                                   |-- Collect Field Errors:             |
          |                                   |   - Field "Port": Invalid Range     |
          |                                   |   - Field "DB": Invalid Scheme      |
          |                                   |                                     |
          |--- Result() --------------------->|                                     |
          |                                   |-- If errors > 0:                    |
          |                                   |   errors.Join(ValidationError...)   |
          |<-- Aggregated Multi-Error --------|                                     |
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                            CONFIG VALIDATOR                             |
 |                                                                         |
 |  Internal State:                                                        |
 |  - errors []ValidationError                                             |
 |                                                                         |
 |  Validation Rules Engine:                                               |
 |  +-> RequireNotEmpty(field, value):                                     |
 |      Check strings.TrimSpace(value) != ""                               |
 |  +-> ValidatePort(field, port):                                         |
 |      Check port >= 1 && port <= 65535                                   |
 |  +-> ValidatePositiveDuration(field, d):                                |
 |      Check d > 0                                                        |
 |  +-> ValidateURL(field, rawURL, allowedSchemes...):                     |
 |      Parse url.ParseRequestURI -> Verify Scheme in allowedSchemes       |
 |  +-> ValidateEnum(field, value, allowed...):                            |
 |      Check value in allowed slice                                       |
 |                                                                         |
 |  Result Aggregation:                                                    |
 |  - Result() -> Returns errors.Join(all validation errors) or nil        |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Validate Semantic Correctness, Not Just Types**: A port may parse successfully as an `int`, but it must also fall strictly within `1..65535`. A URL may be a string, but its scheme must match `https` or `postgres`.
- **Collect All Errors Before Failing**: Avoid early returns on the first failing field. Running the complete validation suite allows operators to fix all invalid settings in a single iteration.
- **Enforce Strict URL Schemes**: When validating database or API endpoints, restrict allowed protocols (e.g., only `postgres` or `https`) to prevent accidental insecure bindings.
- **Trim Whitespace on Required Strings**: Always check `strings.TrimSpace(value)` to prevent whitespace-only strings from bypassing required validation checks.

### Common Pitfalls & Anti-Patterns
- **Ad-Hoc, Inconsistent Checks**: Writing manual `if cfg.Port <= 0` in some services while forgetting duration checks in others, leading to inconsistent startup guarantees.
- **Discarding Field Context**: Returning generic error messages like `"invalid duration"` without specifying which field failed (`"field 'ReadTimeout': duration must be strictly positive"`).
- **Ignoring Whitespace**: Checking `if str != ""` without trimming spaces, allowing whitespace-only strings (`"   "`) to bypass required field checks.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/02_config/config_validator.go`.

### Core Types & Signatures

```go
package config

import (
	"time"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string

type ConfigValidator struct {
	// unexported fields: errors
}

func NewConfigValidator() *ConfigValidator
func (v *ConfigValidator) RequireNotEmpty(field, value string)
func (v *ConfigValidator) ValidatePort(field string, port int)
func (v *ConfigValidator) ValidatePositiveDuration(field string, d time.Duration)
func (v *ConfigValidator) ValidateURL(field, rawURL string, allowedSchemes ...string)
func (v *ConfigValidator) ValidateEnum(field, value string, allowed ...string)
func (v *ConfigValidator) Result() error
```

### Complete End-to-End Example

```go
package main

import (
	"log"
	"time"

	"patterns/02_config"
)

type DatabaseConfig struct {
	Host        string
	Port        int
	URL         string
	Environment string
	Timeout     time.Duration
}

func ValidateDatabaseConfig(cfg DatabaseConfig) error {
	validator := config.NewConfigValidator()

	validator.RequireNotEmpty("Host", cfg.Host)
	validator.ValidatePort("Port", cfg.Port)
	validator.ValidateURL("URL", cfg.URL, "postgres", "postgresql")
	validator.ValidateEnum("Environment", cfg.Environment, "dev", "staging", "prod")
	validator.ValidatePositiveDuration("Timeout", cfg.Timeout)

	return validator.Result()
}

func main() {
	cfg := DatabaseConfig{
		Host:        "localhost",
		Port:        5432,
		URL:         "postgres://user:pass@localhost:5432/db",
		Environment: "prod",
		Timeout:     5 * time.Second,
	}

	if err := ValidateDatabaseConfig(cfg); err != nil {
		log.Fatalf("Invalid configuration detected: %v", err)
	}

	log.Println("Database configuration is valid!")
}
```
