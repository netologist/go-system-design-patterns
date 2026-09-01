package testingpattern_test

import (
	"math/rand/v2"
	"testing"

	testingpattern "system-design-patterns/patterns/18_testing"
)

func TestBankingLedger_TotalBalanceInvariantHolds(t *testing.T) {
	initialBalances := map[string]int64{
		"Alice": 100000,
		"Bob":   100000,
		"Carol": 100000,
		"David": 100000,
	}

	ledger := testingpattern.NewBankingLedger(initialBalances)
	initialTotal := ledger.TotalSystemBalance() // 400,000

	accounts := []string{"Alice", "Bob", "Carol", "David"}

	// Property Test: Execute 200 random transfers
	for range 200 {
		fromIdx := rand.IntN(len(accounts))
		toIdx := rand.IntN(len(accounts))
		from := accounts[fromIdx]
		to := accounts[toIdx]

		randomAmount := int64(rand.IntN(5000) + 1) // 1 to 5000 cents

		_ = ledger.Transfer(from, to, randomAmount)

		// Invariant MUST hold after every single operation:
		currentTotal := ledger.TotalSystemBalance()
		if currentTotal != initialTotal {
			t.Fatalf("INVARIANT VIOLATION: system total changed! initial=%d, current=%d",
				initialTotal, currentTotal)
		}
	}
}
