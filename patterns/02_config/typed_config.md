# Typed Config Pattern

## 1. Overview & Concept
The **Typed Config** pattern parses raw, untyped environment variables and external configuration sources into strongly-typed, immutable Go structs during application bootstrap.

Instead of reading raw string values via `os.Getenv()` throughout the codebase at runtime, the application centralizes environment parsing into a single factory function (`LoadFromEnv`). This enforces type safety (e.g., converting strings to integers, `time.Duration`, custom enums), applies sane production defaults, and accumulates all parsing and missing configuration errors into an aggregated report before the service starts.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Scattering `os.Getenv()` throughout application code causes severe production outages:

- **Late Runtime Panics / Type Parse Failures**: An environment variable like `CACHE_TTL="30s"` or `PORT="8080"` is read deep within a handler. If a deployment config typo occurs (`PORT="808O"` with the letter O), the application crashes mid-request handling on invalid conversions.
- **Silent Misconfigurations via Bad Defaults**: Without explicit parsing and validation, missing environment variables fall back to zero-values (e.g., `0` seconds timeout or empty database host string), leading to silent connection drops or infinite blocking.
- **Hidden Dependencies Across Packages**: Reading environment variables inside deep business services tightly couples domain logic to the deployment environment, preventing automated unit tests from testing alternative configurations.
- **Trial-and-Error Deployment Debugging**: When an application crashes on the first missing environment variable, operators fix it and redeploy, only for it to crash on the second missing variable. Aggregating all errors upfront eliminates this deployment friction.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Configuration Loading Pipeline

```
Environment Variables / OS           EnvGetter Func             LoadFromEnv Factory               ServerConfig Struct
         |                                 |                             |                                  |
         |--- APP_ENV="prod" ------------->|                             |                                  |
         |--- APP_PORT="8080" ------------>|                             |                                  |
         |--- DATABASE_URL="postgres://..."|                             |                                  |
         |--- APP_READ_TIMEOUT="5s" ------>|                             |                                  |
         |                                 |--- Lookup Values ---------->|                                  |
         |                                 |                             |-- Parse & Type Check:            |
         |                                 |                             |   - Environment (dev/staging/prod|
         |                                 |                             |   - Port (strconv.Atoi)          |
         |                                 |                             |   - ReadTimeout (time.ParseDur.) |
         |                                 |                             |   - WriteTimeout (time.ParseDur.)|
         |                                 |                             |   - DatabaseURL (Required != "") |
         |                                 |                             |                                  |
         |                                 |                             |-- Collect Errors (errors.Join)   |
         |                                 |                             |                                  |
         |                                 |                             +--------[Any Errors?]-------------+
         |                                 |                             |                 |                |
         |                                 |                         [Errors]          [Success]            |
         |                                 |                             |                 |                |
         |                                 |                        Return errs     Return &ServerConfig -->|
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                            TYPED CONFIG LOADER                          |
 |                                                                         |
 |  Input:                                                                 |
 |  - EnvGetter: func(key string) string (injectable os.Getenv)            |
 |                                                                         |
 |  Parsing & Type Mapping Rules:                                          |
 |  +-> APP_ENV          -> Environment enum (dev | staging | prod)        |
 |  +-> APP_PORT         -> int (Default: 8080, Range: 1..65535)           |
 |  +-> DATABASE_URL     -> string (Mandatory / Non-empty)                 |
 |  +-> APP_READ_TIMEOUT  -> time.Duration (Default: 5s, time.ParseDuration)|
 |  +-> APP_WRITE_TIMEOUT -> time.Duration (Default: 10s, time.ParseDuration)|
 |  +-> APP_MAX_WORKERS  -> int (Default: 10, Range: > 0)                  |
 |                                                                         |
 |  Error Aggregation:                                                     |
 |  - Appends all formatting, range, and missing variable errors to []error|
 |  - Returns errors.Join(errs...) if len(errs) > 0                        |
 |  - Returns fully populated, validated *ServerConfig on success          |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Parse Once at Startup**: Load and validate configuration only in `main.go` / Composition Root and pass the resulting typed struct down into constructors.
- **Use Native `time.Duration` Types**: Store timeout and interval settings as `time.Duration` rather than raw integer milliseconds or seconds to eliminate unit ambiguity.
- **Fail Fast with Aggregated Errors**: Report every missing or invalid environment variable simultaneously so operators do not have to debug errors one variable at a time across multiple restarts.
- **Injectable Env Lookup (`EnvGetter`)**: Define an `EnvGetter` type (`func(key string) string`) to allow mocking environment maps during unit tests without modifying the process-level environment.

### Common Pitfalls & Anti-Patterns
- **Calling `os.Getenv()` in Domain Logic**: Injecting raw environment reads into business services creates hidden coupling and prevents parallel testing.
- **Permissive Zero-Value Fallbacks**: Defaulting missing timeouts to `0` instead of a sensible default (e.g., `5 * time.Second`), leading to indefinite blocking.
- **Silent Defaults for Critical Secrets**: Providing default credentials for production databases or signing keys in code rather than requiring them explicitly.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/02_config/typed_config.go`.

### Core Types & Signatures

```go
package config

import (
	"time"
)

type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

type ServerConfig struct {
	Env          Environment
	Port         int
	DatabaseURL  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxWorkers   int
}

type EnvGetter func(key string) string

func LoadFromEnv(get EnvGetter) (*ServerConfig, error)
```

### Complete End-to-End Example

```go
package main

import (
	"log"
	"os"

	"patterns/02_config"
)

func main() {
	// 1. Set environment variables for demonstration
	_ = os.Setenv("APP_ENV", "prod")
	_ = os.Setenv("APP_PORT", "9000")
	_ = os.Setenv("DATABASE_URL", "postgres://user:secret@localhost:5432/mydb?sslmode=disable")
	_ = os.Setenv("APP_READ_TIMEOUT", "10s")
	_ = os.Setenv("APP_WRITE_TIMEOUT", "15s")
	_ = os.Setenv("APP_MAX_WORKERS", "25")

	// 2. Load and validate strongly typed configuration at startup
	cfg, err := config.LoadFromEnv(os.Getenv)
	if err != nil {
		log.Fatalf("Fatal configuration error: %v", err)
	}

	log.Printf("Loaded config: Env=%s, Port=%d, DB=%s, ReadTimeout=%v, Workers=%d",
		cfg.Env, cfg.Port, cfg.DatabaseURL, cfg.ReadTimeout, cfg.MaxWorkers)
}
```
