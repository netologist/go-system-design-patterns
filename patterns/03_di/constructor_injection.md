# Constructor Injection Pattern

## 1. Overview & Concept
The **Constructor Injection** pattern is the foundational dependency injection mechanism in idiomatic Go. Dependencies required by a struct (repositories, external API clients, notification dispatchers, loggers) are explicitly declared as parameters in a constructor function (conventionally named `New<TypeName>`).

The constructor validates that all required dependencies are non-nil before returning the initialized struct pointer. This guarantees that an instantiated service is always in a complete, valid, and operational state, eliminating runtime nil-pointer panics and hidden side effects.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Relying on implicit dependencies, global singletons, or setter injection causes severe design and runtime issues:

- **Runtime Nil Pointer Panics**: Creating a struct via bare instantiation (`svc := &UserService{}`) without setting its internal repository causes a panic on the first customer request calling `svc.repo.FindByID()`.
- **Hidden Coupling via Global State**: Functions calling global database variables (e.g., `db.GetDB()`) cannot be tested in isolation and prevent running test suites concurrently with `t.Parallel()`.
- **Temporal Coupling with Setters**: Setter injection (`svc.SetRepo(repo)`) creates temporal ordering dependencies where calling methods before setters causes subtle panics or invalid states.
- **Inability to Mock Dependencies**: Structs that instantiate their own internal dependencies (`svc.repo = NewPostgresRepo()`) cannot be unit-tested with mock or fake repositories.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Constructor Dependency Injection Flow

```
Caller / Composition Root                 NewUserService(repo, notifier)               UserService Instance
            |                                           |                                      |
            |--- NewUserService(repo, notifier) ------->|                                      |
            |                                           |-- 1. Check repo == nil               |
            |                                           |-- 2. Check notifier == nil           |
            |                                           |                                      |
            |                                           +----------[Are Any Nil?]--------------+
            |                                           |                     |                |
            |                                         [Yes]                  [No]              |
            |                                           |                     |                |
            |<-- Return Error ("cannot be nil") --------|             Instantiate Struct       |
            |                                                                 |                |
            |<-- Return (*UserService, nil) ----------------------------------+--------------->|
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                      CONSTRUCTOR INJECTION ARCHITECTURE                 |
 |                                                                         |
 |  Consumer Service: UserService                                          |
 |  - repo: UserRepository (Interface)                                    |
 |  - notifier: NotificationSender (Interface)                             |
 |                                                                         |
 |  Constructor: NewUserService(repo, notifier) (*UserService, error)      |
 |  1. Enforce Non-Nil Preconditions:                                      |
 |     +--> If repo == nil     -> return nil, errors.New("cannot be nil")  |
 |     +--> If notifier == nil -> return nil, errors.New("cannot be nil")  |
 |  2. Initialize Unexported Fields                                        |
 |  3. Return Fully Functional, Validated Pointer                          |
 |                                                                         |
 |  Benefits:                                                              |
 |  - Compile-time interface verification                                  |
 |  - 100% guarantee of non-nil fields during runtime execution           |
 |  - Thread-safe for parallel test execution                              |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Return Errors from Constructors**: If dependencies are mandatory, return `(*Service, error)` from the constructor and reject `nil` parameters with meaningful error messages.
- **Accept Interfaces, Return Concrete Structs**: Accept interfaces in constructor parameters so callers can substitute mocks or alternative implementations, but return concrete struct pointers (e.g., `*UserService`).
- **Keep Dependencies Unexported**: Store injected dependencies in unexported struct fields (`repo`, `notifier`) to prevent external packages from tampering with them post-construction.
- **Do Not Start Goroutines in Constructors**: Keep constructors synchronous and fast. Start background workers explicitly in a separate `Start(ctx)` or `Run(ctx)` method.

### Common Pitfalls & Anti-Patterns
- **Ignoring Nil Parameter Checks**: Creating constructors that assign parameters directly without checking for `nil`, which defeats the safety guarantees of constructor injection.
- **Instantiating Dependencies Inside the Constructor**: Calling `repo := NewPostgresRepo()` inside `NewUserService()` hardcodes the implementation and prevents injecting mock repositories for testing.
- **Service Locator Anti-Pattern**: Passing a generic container or context (e.g., `NewUserService(container *DIContainer)`) into the constructor, which hides the actual dependencies required by the service.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/03_di/constructor_injection.go`.

### Core Types & Signatures

```go
package di

import "context"

type User struct {
	ID    string
	Email string
	Name  string
}

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, user *User) error
}

type NotificationSender interface {
	SendWelcomeEmail(ctx context.Context, email, name string) error
}

type UserService struct {
	// unexported fields: repo, notifier
}

func NewUserService(repo UserRepository, notifier NotificationSender) (*UserService, error)
func (s *UserService) RegisterUser(ctx context.Context, user *User) error
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"log"

	"patterns/03_di"
)

// In-memory test mock for UserRepository
type MockUserRepo struct {
	users map[string]*di.User
}

func (m *MockUserRepo) FindByID(ctx context.Context, id string) (*di.User, error) {
	return m.users[id], nil
}

func (m *MockUserRepo) Save(ctx context.Context, user *di.User) error {
	m.users[user.ID] = user
	return nil
}

// In-memory test mock for NotificationSender
type MockNotifier struct{}

func (m *MockNotifier) SendWelcomeEmail(ctx context.Context, email, name string) error {
	log.Printf("Mock email sent to %s (%s)", name, email)
	return nil
}

func main() {
	repo := &MockUserRepo{users: make(map[string]*di.User)}
	notifier := &MockNotifier{}

	// 1. Explicit injection with non-nil validation
	svc, err := di.NewUserService(repo, notifier)
	if err != nil {
		log.Fatalf("Failed to construct UserService: %v", err)
	}

	// 2. Execute business operations
	ctx := context.Background()
	user := &di.User{ID: "usr_1", Email: "alex@example.com", Name: "Alex"}

	if err := svc.RegisterUser(ctx, user); err != nil {
		log.Fatalf("Registration failed: %v", err)
	}

	log.Println("User registered and notified successfully.")
}
```
