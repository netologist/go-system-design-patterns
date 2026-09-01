package di

import (
	"context"
	"errors"
	"sync"
)

// Account represents a bank account.
type Account struct {
	ID      string
	Balance int64
}

// AccountRepository represents account persistence.
type AccountRepository interface {
	Get(ctx context.Context, id string) (*Account, error)
	Save(ctx context.Context, acc *Account) error
}

// 1. Fake: In-memory working implementation with real storage semantics.
type FakeAccountRepository struct {
	mu       sync.RWMutex
	accounts map[string]*Account
}

func NewFakeAccountRepository() *FakeAccountRepository {
	return &FakeAccountRepository{
		accounts: make(map[string]*Account),
	}
}

func (f *FakeAccountRepository) Get(ctx context.Context, id string) (*Account, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	acc, ok := f.accounts[id]
	if !ok {
		return nil, errors.New("account not found")
	}
	// Return copy to prevent race condition
	return &Account{ID: acc.ID, Balance: acc.Balance}, nil
}

func (f *FakeAccountRepository) Save(ctx context.Context, acc *Account) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accounts[acc.ID] = &Account{ID: acc.ID, Balance: acc.Balance}
	return nil
}

// 2. Stub: Returns canned answers regardless of input.
type StubAccountRepository struct {
	CannedAccount *Account
	CannedError   error
}

func (s *StubAccountRepository) Get(ctx context.Context, id string) (*Account, error) {
	return s.CannedAccount, s.CannedError
}

func (s *StubAccountRepository) Save(ctx context.Context, acc *Account) error {
	return s.CannedError
}

// 3. Spy / Mock: Records invocations and parameters for assertion.
type SpyAccountRepository struct {
	mu           sync.Mutex
	GetCalls     []string
	SaveCalls    []*Account
	ReturnError  error
}

func (m *SpyAccountRepository) Get(ctx context.Context, id string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCalls = append(m.GetCalls, id)
	if m.ReturnError != nil {
		return nil, m.ReturnError
	}
	return &Account{ID: id, Balance: 1000}, nil
}

func (m *SpyAccountRepository) Save(ctx context.Context, acc *Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveCalls = append(m.SaveCalls, acc)
	return m.ReturnError
}
