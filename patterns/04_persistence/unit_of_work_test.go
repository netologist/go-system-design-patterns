package persistence_test

import (
	"context"
	"errors"
	"testing"

	persistence "system-design-patterns/patterns/04_persistence"
)

func TestUnitOfWork_CommitSuccess(t *testing.T) {
	uow := persistence.NewInMemoryUnitOfWork()
	uow.SeedWallet(&persistence.Wallet{ID: "w-1", Balance: 1000})

	err := uow.ExecuteTx(context.Background(), func(tx persistence.TxScope) error {
		w, err := tx.GetWallet(context.Background(), "w-1")
		if err != nil {
			return err
		}
		w.Balance -= 200
		if err := tx.UpdateWallet(context.Background(), w); err != nil {
			return err
		}
		return tx.CreateLedgerEntry(context.Background(), &persistence.LedgerEntry{
			ID:        "led-1",
			WalletID:  "w-1",
			Amount:    -200,
			Reference: "Payment #1",
		})
	})

	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	bal, _ := uow.GetBalance("w-1")
	if bal != 800 {
		t.Errorf("expected balance 800 after commit, got: %d", bal)
	}

	if uow.LedgerCount() != 1 {
		t.Errorf("expected 1 ledger entry, got: %d", uow.LedgerCount())
	}
}

func TestUnitOfWork_RollbackOnError(t *testing.T) {
	uow := persistence.NewInMemoryUnitOfWork()
	uow.SeedWallet(&persistence.Wallet{ID: "w-1", Balance: 1000})

	err := uow.ExecuteTx(context.Background(), func(tx persistence.TxScope) error {
		w, _ := tx.GetWallet(context.Background(), "w-1")
		w.Balance -= 500
		_ = tx.UpdateWallet(context.Background(), w)

		// Fail on ledger step
		return errors.New("ledger service unavailable")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	bal, _ := uow.GetBalance("w-1")
	if bal != 1000 {
		t.Errorf("expected balance 1000 preserved after rollback, got: %d", bal)
	}

	if uow.LedgerCount() != 0 {
		t.Errorf("expected 0 ledger entries after rollback, got: %d", uow.LedgerCount())
	}
}
