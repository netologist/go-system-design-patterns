package dataintegrity

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNegativeBalance    = errors.New("domain invariant violation: account balance cannot be negative")
	ErrEmptyAccountOwner  = errors.New("domain invariant violation: account owner cannot be empty")
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
