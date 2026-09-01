package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Wallet represents a customer balance.
type Wallet struct {
	ID      string
	Balance int64
}

// LedgerEntry represents an audit ledger log.
type LedgerEntry struct {
	ID        string
	WalletID  string
	Amount    int64
	Reference string
}

// TxScope provides access to repositories participating in a transaction.
type TxScope interface {
	GetWallet(ctx context.Context, id string) (*Wallet, error)
	UpdateWallet(ctx context.Context, w *Wallet) error
	CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) error
}

// UnitOfWork manages the transaction lifecycle.
type UnitOfWork interface {
	ExecuteTx(ctx context.Context, fn func(tx TxScope) error) error
}

// InMemoryUnitOfWork simulates transaction rollback/commit using copy-on-write.
type InMemoryUnitOfWork struct {
	mu      sync.RWMutex
	wallets map[string]*Wallet
	ledgers []LedgerEntry
}

func NewInMemoryUnitOfWork() *InMemoryUnitOfWork {
	return &InMemoryUnitOfWork{
		wallets: make(map[string]*Wallet),
		ledgers: make([]LedgerEntry, 0),
	}
}

// SeedWallet helper for testing setup.
func (u *InMemoryUnitOfWork) SeedWallet(w *Wallet) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.wallets[w.ID] = &Wallet{ID: w.ID, Balance: w.Balance}
}

func (u *InMemoryUnitOfWork) GetBalance(id string) (int64, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	w, ok := u.wallets[id]
	if !ok {
		return 0, errors.New("wallet not found")
	}
	return w.Balance, nil
}

func (u *InMemoryUnitOfWork) LedgerCount() int {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return len(u.ledgers)
}

type txSession struct {
	parent         *InMemoryUnitOfWork
	stagedWallets  map[string]*Wallet
	stagedLedgers  []LedgerEntry
}

func (s *txSession) GetWallet(ctx context.Context, id string) (*Wallet, error) {
	if w, ok := s.stagedWallets[id]; ok {
		return &Wallet{ID: w.ID, Balance: w.Balance}, nil
	}
	s.parent.mu.RLock()
	defer s.parent.mu.RUnlock()
	w, ok := s.parent.wallets[id]
	if !ok {
		return nil, errors.New("wallet not found")
	}
	// Stage a copy
	staged := &Wallet{ID: w.ID, Balance: w.Balance}
	s.stagedWallets[id] = staged
	return &Wallet{ID: staged.ID, Balance: staged.Balance}, nil
}

func (s *txSession) UpdateWallet(ctx context.Context, w *Wallet) error {
	if w == nil || w.ID == "" {
		return errors.New("invalid wallet")
	}
	s.stagedWallets[w.ID] = &Wallet{ID: w.ID, Balance: w.Balance}
	return nil
}

func (s *txSession) CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) error {
	if entry == nil {
		return errors.New("invalid ledger entry")
	}
	s.stagedLedgers = append(s.stagedLedgers, *entry)
	return nil
}

// ExecuteTx wraps operations in a transaction, rolling back staged changes on error/panic.
func (u *InMemoryUnitOfWork) ExecuteTx(ctx context.Context, fn func(tx TxScope) error) (err error) {
	session := &txSession{
		parent:        u,
		stagedWallets: make(map[string]*Wallet),
		stagedLedgers: make([]LedgerEntry, 0),
	}

	// Safe panic handling
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("transaction aborted due to panic: %v", r)
		}
	}()

	if err = fn(session); err != nil {
		// Rollback: staged changes are simply discarded
		return fmt.Errorf("transaction rolled back: %w", err)
	}

	// Commit: apply staged changes atomically under lock
	u.mu.Lock()
	defer u.mu.Unlock()

	for k, v := range session.stagedWallets {
		u.wallets[k] = v
	}
	u.ledgers = append(u.ledgers, session.stagedLedgers...)

	return nil
}
