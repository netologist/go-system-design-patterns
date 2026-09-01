# Test Doubles Pattern (Fakes, Stubs, Spies & Mocks)

## 1. Overview & Concept
The **Test Doubles** pattern encompasses the taxonomy and implementation of lightweight replacement objects used in unit and integration testing to isolate business logic from slow, non-deterministic, or stateful external systems (databases, payment processors, external APIs).

Following Gerard Meszaros' classic test taxonomy, Go test doubles are categorized into:
1. **Fake**: A working, stateful in-memory implementation (e.g., an in-memory repository backed by a mutex-protected `map[string]*T`) that mimics real persistence semantics.
2. **Stub**: Provides hardcoded, canned responses to method calls without any state management or logic.
3. **Spy / Mock**: Records method invocations, parameter values, and call counts for post-execution assertions.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Running tests against real external dependencies or poorly designed test doubles leads to severe testing problems:

- **Flaky & Slow Test Suites**: Hitting real network services or disk databases introduces network latency, timeouts, and shared state collisions, making CI builds slow and non-deterministic.
- **Over-Mocking Fragility**: Using heavy dynamic reflection-based mocking frameworks (which assert every single internal function call order) causes tests to break upon the slightest refactor, even when business logic remains 100% correct.
- **Data Races in Test Fakes**: Using bare Go maps without synchronization in test fakes causes data races and test crashes when tested against concurrent code (`t.Parallel()`, `go test -race`).
- **Memory Pointer Leaks in Fakes**: Returning internal pointers from fake storage allows tests to mutate fake memory directly without going through the repository methods, creating false-positive tests.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Test Double Taxonomy

```
+---------------------------------------------------------------------------------------------------+
|                                      TEST DOUBLE TAXONOMY                                         |
+---------------------+-------------------------------+---------------------------------------------+
| Type                | Mechanism                     | Production Use Case                         |
+---------------------+-------------------------------+---------------------------------------------+
| 1. Fake             | In-Memory State + Mutex       | Full CRUD workflows, multi-step transactions|
|                     | Clones data on read & write   | Integration & Domain Service Tests          |
+---------------------+-------------------------------+---------------------------------------------+
| 2. Stub             | Returns canned values/errors  | Simulating network failure or edge cases    |
|                     | Zero state, pure static data  | Testing error handling paths                |
+---------------------+-------------------------------+---------------------------------------------+
| 3. Spy / Mock       | Records []CallParameters      | Verifying side-effects (e.g., email sent)   |
|                     | Mutex-protected invocation log| Asserting specific arguments were passed    |
+---------------------+-------------------------------+---------------------------------------------+
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                         FAKE REPOSITORY ARCHITECTURE                    |
 |                                                                         |
 |  FakeAccountRepository:                                                 |
 |  - mu: sync.RWMutex                                                     |
 |  - accounts: map[string]*Account                                        |
 |                                                                         |
 |  Thread-Safe Operations with Defensive Copying:                         |
 |  +-> Get(ctx, id) (*Account, error):                                    |
 |      f.mu.RLock(); defer f.mu.RUnlock()                                 |
 |      acc, ok := f.accounts[id]                                          |
 |      if !ok -> return nil, ErrNotFound                                  |
 |      return &Account{ID: acc.ID, Balance: acc.Balance}, nil // CLONE!   |
 |                                                                         |
 |  +-> Save(ctx, acc) error:                                              |
 |      f.mu.Lock(); defer f.mu.Unlock()                                   |
 |      f.accounts[acc.ID] = &Account{ID: acc.ID, Balance: acc.Balance}    |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Prefer Fakes Over Mocks for Repositories**: An in-memory fake repository with thread safety provides realistic stateful behavior across complex transaction flows without brittle call expectations.
- **Defensive Copying in Fakes**: When reading from or writing to internal maps in a fake, return a cloned copy of the struct to prevent caller mutations from causing race conditions.
- **Synchronize Shared State with Mutexes**: Always protect internal slices and maps in fakes and spies using `sync.RWMutex` so tests can run safely under `go test -race` and `t.Parallel()`.
- **Keep Test Doubles in `_test.go` or Internal Test Packages**: Test doubles should not be exported in production binary releases unless explicitly distributed in an `export_test.go` or `testing` subpackage.

### Common Pitfalls & Anti-Patterns
- **Unsynchronized Maps in Fakes**: Using bare `map[string]*Account` without mutexes in a test fake, causing data races when tested against concurrent code.
- **Returning Internal Pointers from Fakes**: Returning `return f.accounts[id]` directly. The caller modifies the struct in-place, silently mutating the fake's internal state without calling `.Save()`.
- **Coupling Tests to Implementation Details**: Asserting the exact order and number of private internal calls rather than verifying observable state outcomes and return values.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/03_di/test_doubles.go`.

### Core Types & Signatures

```go
package di

import (
	"context"
	"sync"
)

type Account struct {
	ID      string
	Balance int64
}

type AccountRepository interface {
	Get(ctx context.Context, id string) (*Account, error)
	Save(ctx context.Context, acc *Account) error
}

// 1. Fake
type FakeAccountRepository struct {
	mu       sync.RWMutex
	accounts map[string]*Account
}

func NewFakeAccountRepository() *FakeAccountRepository
func (f *FakeAccountRepository) Get(ctx context.Context, id string) (*Account, error)
func (f *FakeAccountRepository) Save(ctx context.Context, acc *Account) error

// 2. Stub
type StubAccountRepository struct {
	CannedAccount *Account
	CannedError   error
}

func (s *StubAccountRepository) Get(ctx context.Context, id string) (*Account, error)
func (s *StubAccountRepository) Save(ctx context.Context, acc *Account) error

// 3. Spy
type SpyAccountRepository struct {
	mu          sync.Mutex
	GetCalls    []string
	SaveCalls   []*Account
	ReturnError error
}

func (m *SpyAccountRepository) Get(ctx context.Context, id string) (*Account, error)
func (m *SpyAccountRepository) Save(ctx context.Context, acc *Account) error
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"errors"
	"log"

	"patterns/03_di"
)

func TransferFunds(ctx context.Context, repo di.AccountRepository, fromID, toID string, amount int64) error {
	from, err := repo.Get(ctx, fromID)
	if err != nil {
		return err
	}
	if from.Balance < amount {
		return errors.New("insufficient funds")
	}

	to, err := repo.Get(ctx, toID)
	if err != nil {
		return err
	}

	from.Balance -= amount
	to.Balance += amount

	if err := repo.Save(ctx, from); err != nil {
		return err
	}
	return repo.Save(ctx, to)
}

func main() {
	ctx := context.Background()

	// 1. Testing with a Fake Repository (Stateful validation)
	fakeRepo := di.NewFakeAccountRepository()
	_ = fakeRepo.Save(ctx, &di.Account{ID: "acc_1", Balance: 500})
	_ = fakeRepo.Save(ctx, &di.Account{ID: "acc_2", Balance: 100})

	if err := TransferFunds(ctx, fakeRepo, "acc_1", "acc_2", 200); err != nil {
		log.Fatalf("Transfer failed: %v", err)
	}

	acc1, _ := fakeRepo.Get(ctx, "acc_1")
	acc2, _ := fakeRepo.Get(ctx, "acc_2")
	log.Printf("Fake verification: Acc1 Balance=%d (Expected 300), Acc2 Balance=%d (Expected 300)",
		acc1.Balance, acc2.Balance)

	// 2. Testing error handling with a Stub
	stubRepo := &di.StubAccountRepository{
		CannedError: errors.New("database connection refused"),
	}
	if err := TransferFunds(ctx, stubRepo, "acc_1", "acc_2", 100); err != nil {
		log.Printf("Stub verification (handled expected error): %v", err)
	}

	// 3. Verifying method invocations with a Spy
	spyRepo := &di.SpyAccountRepository{}
	_ = spyRepo.Save(ctx, &di.Account{ID: "acc_3", Balance: 1000})
	log.Printf("Spy verification: Save called %d times", len(spyRepo.SaveCalls))
}
```
