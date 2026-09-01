package dataintegrity_test

import (
	"errors"
	"testing"

	dataintegrity "system-design-patterns/patterns/23_dataintegrity"
)

func TestInvariantBankAccount_AuditAndOverdraftGuards(t *testing.T) {
	// 1. Initial negative balance rejected
	_, err := dataintegrity.NewInvariantBankAccount("acc-1", "Hasan", -50)
	if !errors.Is(err, dataintegrity.ErrNegativeBalance) {
		t.Errorf("expected ErrNegativeBalance for negative initial balance, got: %v", err)
	}

	// 2. Normal account creation
	acc, err := dataintegrity.NewInvariantBankAccount("acc-1", "Hasan", 1000)
	if err != nil {
		t.Fatalf("account creation failed: %v", err)
	}

	// 3. Valid withdrawal
	err = acc.Withdraw("teller_1", 300)
	if err != nil || acc.Balance() != 700 {
		t.Errorf("withdrawal failed, balance: %d", acc.Balance())
	}

	// 4. Overdraft withdrawal rejected (invariant protected)
	err = acc.Withdraw("teller_1", 800)
	if !errors.Is(err, dataintegrity.ErrNegativeBalance) {
		t.Errorf("expected overdraft rejected by invariant, got: %v", err)
	}

	// 5. Audit trail records
	audit := acc.AuditTrail()
	if len(audit) != 2 { // Init + 1 successful withdrawal
		t.Errorf("expected 2 audit log entries, got: %d", len(audit))
	}
}
