package testingpattern

import (
	"errors"
	"sync"
)

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidTransfer   = errors.New("transfer amount must be strictly positive")
	ErrSameAccount       = errors.New("source and destination accounts must be different")
)

// BankingLedger maintains bank account balances and executes atomic transfers.
type BankingLedger struct {
	mu       sync.Mutex
	balances map[string]int64
}

func NewBankingLedger(initialBalances map[string]int64) *BankingLedger {
	b := make(map[string]int64, len(initialBalances))
	for k, v := range initialBalances {
		b[k] = v
	}
	return &BankingLedger{balances: b}
}

// Transfer transfers amount from source to destination.
func (l *BankingLedger) Transfer(from, to string, amount int64) error {
	if amount <= 0 {
		return ErrInvalidTransfer
	}
	if from == to {
		return ErrSameAccount
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	fromBal, okFrom := l.balances[from]
	_, okTo := l.balances[to]
	if !okFrom || !okTo {
		return errors.New("account does not exist")
	}
	if fromBal < amount {
		return ErrInsufficientFunds
	}

	l.balances[from] -= amount
	l.balances[to] += amount
	return nil
}

// TotalSystemBalance calculates the sum of all accounts (the core invariant).
func (l *BankingLedger) TotalSystemBalance() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	var total int64
	for _, bal := range l.balances {
		total += bal
	}
	return total
}
