# Domain Invariants & Audit Trail Pattern

## Overview & Definition
The **Domain Invariants & Audit Trail Pattern** encapsulates core business rules, entity boundary validation, and immutable event logging directly inside Domain-Driven Design (DDD) aggregates.

An **invariant** is a business assertion that must always hold true throughout the entire lifecycle of an entity (e.g., *a bank account balance must never drop below zero*, *an account must always have an assigned owner*). By forbidding direct external mutation of aggregate fields (encapsulation) and forcing all state mutations through validated domain methods (`Withdraw`, `Deposit`), the aggregate guarantees business correctness while automatically generating a cryptographic or chronological **Audit Trail**.

---

## Problem Statement
Exposing entity fields directly to controllers or data mappers leads to scattered business logic, invariant violations, and unaccountable data corruption.

### Failure Scenarios Without This Pattern
- **Overdraft & Race-Condition Exploits:** Simultaneous ATM or API withdrawal requests bypass checks if business logic is scattered across handlers, resulting in negative account balances.
- **Unaccountable Data Mutations (No Audit Log):** Balances change in the database without any record of *who* initiated the change, *why* it occurred, or *what* the previous balance was, making regulatory compliance (SOX, PCI-DSS) impossible.
- **Anemic Domain Models:** Entities become passive data bags (`struct` with public fields), forcing every API controller to rewrite the same validation rules.
- **Inconsistent Aggregate Creation:** Instantiating entities in invalid initial states (e.g., opening an account with an empty owner or negative starting balance).

---

## Architectural Mechanism & Flow
The `InvariantBankAccount` guards constructor instantiation and wraps every balance mutation with invariant checks and immutable audit logging:

```
[ Client Initiates: Withdraw(Actor="Alice", Amount=50) ]
                           │
                           ▼
            ┌─────────────────────────────┐
            │ InvariantBankAccount.Lock() │
            └──────────────┬──────────────┘
                           │
            ┌──────────────┴──────────────┐
            │   amount <= 0?              │ ──[Yes]──► [ Reject: ErrInvalidAmount ]
            └──────────────┬──────────────┘
                           │ [No]
            ┌──────────────┴──────────────┐
            │   balance - amount < 0?     │ ──[Yes]──► [ Reject: ErrNegativeBalance ]
            └──────────────┬──────────────┘
                           │ [No: Invariant Satisfied]
            ┌──────────────┴──────────────┐
            │ 1. balance -= amount        │
            │ 2. Append AuditEntry:       │
            │    - ID, Actor, Timestamp   │
            │    - Delta: -50             │
            │    - New Balance: 150       │
            └──────────────┬──────────────┘
                           │
                           ▼
            ┌─────────────────────────────┐
            │ InvariantBankAccount.Unlock │
            └──────────────┬──────────────┘
                           │
                           ▼
                 [ Return Success (nil) ]
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Private Fields with Controlled Mutators:** Keep all structural fields (`balance`, `auditLogs`) private. Expose state changes exclusively through explicit, intention-revealing methods (`Withdraw`, `Deposit`).
- **Defensive Copying of Slices:** When exposing historical audit trails (`AuditTrail()`), always copy the internal slice into a new slice to prevent callers from modifying internal slice headers or data.
- **Atomic Concurrency Protection:** Synchronize domain operations with internal mutexes (`sync.Mutex`) or database-level row locking.
- **Enforce Invariants in Factory Constructors:** Never allow constructors (`NewInvariantBankAccount`) to return partially initialized or invalid objects. Return an error immediately if arguments violate invariants.

### Pitfalls to Avoid
- **Leaking Pointers to Internal State:** Returning pointers to mutable internal sub-structures allows external code to bypass domain invariant validation.
- **Separating Audit Logging from Domain Mutations:** Logging audit entries in an external service after the database commit risks audit log loss if the network fails midway.
- **Using Floating Point Numbers for Currency:** Never use `float32` or `float64` for money due to IEEE 754 rounding errors. Always use integer cents (`int64`) or arbitrary-precision decimal libraries.

---

## Code Walkthrough & Usage

### Core Implementation
The pattern implementation in `domain_invariants.go` guarantees invariants and audit trail creation:

```go
package dataintegrity

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNegativeBalance   = errors.New("domain invariant violation: account balance cannot be negative")
	ErrEmptyAccountOwner = errors.New("domain invariant violation: account owner cannot be empty")
)

// AuditEntry records immutable audit trail details.
type AuditEntry struct {
	ID        string
	Actor     string
	Action    string
	AccountID string
	Delta     int64
	Balance   int64
	Timestamp time.Time
}

// InvariantBankAccount enforces financial invariants and appends audit logs on every state change.
type InvariantBankAccount struct {
	mu        sync.Mutex
	id        string
	owner     string
	balance   int64
	auditLogs []AuditEntry
}

func NewInvariantBankAccount(id, owner string, initialBalance int64) (*InvariantBankAccount, error) {
	if owner == "" {
		return nil, ErrEmptyAccountOwner
	}
	if initialBalance < 0 {
		return nil, ErrNegativeBalance
	}

	acc := &InvariantBankAccount{
		id:      id,
		owner:   owner,
		balance: initialBalance,
		auditLogs: []AuditEntry{
			{
				ID:        fmt.Sprintf("audit-%s-init", id),
				Actor:     "SYSTEM",
				Action:    "ACCOUNT_OPENED",
				AccountID: id,
				Delta:     initialBalance,
				Balance:   initialBalance,
				Timestamp: time.Now().UTC(),
			},
		},
	}
	return acc, nil
}

// Withdraw applies withdrawal while strictly enforcing invariant (Balance >= 0).
func (a *InvariantBankAccount) Withdraw(actor string, amount int64) error {
	if amount <= 0 {
		return errors.New("withdrawal amount must be positive")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.balance-amount < 0 {
		return ErrNegativeBalance // Invariant Guard
	}

	a.balance -= amount
	a.auditLogs = append(a.auditLogs, AuditEntry{
		ID:        fmt.Sprintf("audit-%s-%d", a.id, len(a.auditLogs)+1),
		Actor:     actor,
		Action:    "WITHDRAWAL",
		AccountID: a.id,
		Delta:     -amount,
		Balance:   a.balance,
		Timestamp: time.Now().UTC(),
	})

	return nil
}

func (a *InvariantBankAccount) Balance() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.balance
}

func (a *InvariantBankAccount) AuditTrail() []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	res := make([]AuditEntry, len(a.auditLogs))
	copy(res, a.auditLogs)
	return res
}
```

### Production Banking Domain Service Example

```go
func ExecuteTransfer(fromAcc, toAcc *dataintegrity.InvariantBankAccount, amount int64, actor string) error {
    // Both operations must satisfy domain invariants
    if err := fromAcc.Withdraw(actor, amount); err != nil {
        return fmt.Errorf("source account transfer failed: %w", err)
    }

    // Inspect immutable audit trail for compliance
    trail := fromAcc.AuditTrail()
    log.Printf("[AUDIT] Last transaction recorded: %+v", trail[len(trail)-1])
    return nil
}
```
