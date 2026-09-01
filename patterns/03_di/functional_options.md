# Functional Options Pattern

## 1. Overview & Concept
The **Functional Options** pattern provides an elegant, extensible, and backward-compatible API for constructing complex structs (such as HTTP clients, gRPC servers, or database connection managers) with dozens of optional configuration parameters.

Originating from Rob Pike and Dave Cheney, the pattern represents optional configuration as closures:
- A constructor accepts mandatory arguments as regular positional parameters, followed by a variadic slice of option functions (`...ClientOption`).
- Option functions mutate unexported fields on the struct or return an `error` if parameters are invalid.
- Sensible defaults are applied first, and options override defaults in caller-specified order.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Traditional constructor styles in Go present severe maintenance and usability challenges:

- **Telescoping Constructor Anti-Pattern**: Adding new parameters requires creating `NewClient()`, `NewClientWithTimeout()`, `NewClientWithTimeoutAndRetries()`, cluttering the public API and breaking backward compatibility.
- **Config Struct Boilerplate & Zero-Value Ambiguity**: Passing a `Config` struct (`NewClient(baseURL, Config{Timeout: 0})`) makes it difficult to differentiate between a user explicitly requesting a zero timeout vs omitting the field to use the default value.
- **Nil Pointer Arguments**: Passing `nil` pointers or empty structs (`NewClient(url, nil, nil, 10, nil)`) creates ugly, error-prone call sites with positional argument confusion.
- **Lack of Construction Validation**: Struct literal instantiation (`client := &Client{Timeout: -1}`) bypasses validation, allowing invalid parameters into production.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Option Evaluation Pipeline

```
Caller                                            NewClient(baseURL, opts...)                       Client Instance
  |                                                            |                                           |
  |--- NewClient("https://api.com", WithTimeout(5s), ...) ---->|                                           |
  |                                                            |-- 1. Apply Default Production Values:     |
  |                                                            |      - Timeout = 30s                      |
  |                                                            |      - MaxRetries = 3                     |
  |                                                            |      - MaxIdleConns = 100                 |
  |                                                            |                                           |
  |                                                            |-- 2. Iterate & Execute Options:           |
  |                                                            |      +--> opt1: Set Timeout = 5s          |
  |                                                            |      +--> opt2: Validate & Set Retries    |
  |                                                            |                                           |
  |                                                            |-- 3. Check for Option Errors:             |
  |                                                            |      [If error != nil -> Return nil, err] |
  |                                                            |                                           |
  |                                                            |-- 4. Construct Internal http.Transport:   |
  |                                                            |      (Inject timeouts & idle conns)       |
  |                                                            |                                           |
  |<-- Return Initialized (*Client, nil) ----------------------+------------------------------------------->|
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                       FUNCTIONAL OPTIONS ARCHITECTURE                   |
 |                                                                         |
 |  Option Type Signature:                                                 |
 |  type ClientOption func(*Client) error                                  |
 |                                                                         |
 |  Constructor Execution Order:                                           |
 |  1. Validate Mandatory Parameters (e.g. baseURL != "")                  |
 |  2. Instantiate Client with Battle-Tested Defaults:                     |
 |     - Timeout: 30 * time.Second                                         |
 |     - MaxRetries: 3                                                     |
 |     - UserAgent: "Go-Service-Client/1.0"                                |
 |     - MaxIdleConns: 100                                                 |
 |  3. Loop over variadic opts slice:                                      |
 |     for _, opt := range opts {                                          |
 |         if err := opt(c); err != nil { return nil, err }                |
 |     }                                                                   |
 |  4. Construct complex internal sub-structures (http.Transport)          |
 |  5. Return configured, immutable *Client pointer                        |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Return Errors from Option Functions**: Ensure option functions can validate their arguments (`if timeout <= 0 { return errors.New(...) }`) rather than failing silently or panicking.
- **Provide Safe Production Defaults**: Never force callers to specify basic options (e.g., connection timeouts or pool sizes); default to battle-tested production values.
- **Keep Struct Fields Unexported**: Prevent external callers from modifying struct fields directly after construction. Provide read-only accessor methods if inspection is required.
- **Separate Mandatory from Optional Arguments**: Keep strictly mandatory dependencies (e.g., `baseURL`, `db`) as regular positional arguments in the constructor; use functional options only for optional settings.

### Common Pitfalls & Anti-Patterns
- **Ignoring Option Return Errors**: Defining options as `func(*Client)` without returning errors, leaving no mechanism to validate user inputs at construction time.
- **Mutating Shared Global State in Options**: Modifying global variables or package-level state inside an option function rather than restricting mutations to the target struct.
- **Overusing Options for Mandatory Parameters**: Making strictly required arguments (like `baseURL`) an optional functional option instead of an explicit positional argument.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/03_di/functional_options.go`.

### Core Types & Signatures

```go
package di

import "time"

type ClientOption func(*Client) error

type Client struct {
	// unexported fields: httpClient, baseURL, timeout, maxRetries, userAgent, maxIdleConns
}

func WithTimeout(d time.Duration) ClientOption
func WithMaxRetries(retries int) ClientOption
func WithUserAgent(ua string) ClientOption
func WithMaxIdleConns(n int) ClientOption

func NewClient(baseURL string, opts ...ClientOption) (*Client, error)
func (c *Client) BaseURL() string
func (c *Client) Timeout() time.Duration
func (c *Client) MaxRetries() int
func (c *Client) UserAgent() string
func (c *Client) MaxIdleConns() int
```

### Complete End-to-End Example

```go
package main

import (
	"log"
	"time"

	"patterns/03_di"
)

func main() {
	// 1. Construct client using default options
	defaultClient, err := di.NewClient("https://api.example.com")
	if err != nil {
		log.Fatalf("Failed to create default client: %v", err)
	}
	log.Printf("Default Client: Timeout=%v, Retries=%d, UA=%s",
		defaultClient.Timeout(), defaultClient.MaxRetries(), defaultClient.UserAgent())

	// 2. Construct client with custom functional options
	customClient, err := di.NewClient(
		"https://payment.gateway.internal",
		di.WithTimeout(5*time.Second),
		di.WithMaxRetries(5),
		di.WithUserAgent("OrderService/2.4.0"),
		di.WithMaxIdleConns(250),
	)
	if err != nil {
		log.Fatalf("Failed to create custom client: %v", err)
	}

	log.Printf("Custom Client: Timeout=%v, Retries=%d, UA=%s, MaxIdle=%d",
		customClient.Timeout(), customClient.MaxRetries(), customClient.UserAgent(), customClient.MaxIdleConns())

	// 3. Option validation rejects invalid values upfront
	_, err = di.NewClient("https://api.example.com", di.WithMaxRetries(-1))
	if err != nil {
		log.Printf("Correctly rejected invalid option: %v", err)
	}
}
```
