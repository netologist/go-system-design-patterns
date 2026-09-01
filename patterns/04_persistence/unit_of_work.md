# Unit of Work Pattern

## Overview & Definition
The **Unit of Work** pattern maintains a list of business objects affected by a business transaction and coordinates the writing out of changes and the resolution of concurrency problems.

In Go backend systems, the Unit of Work pattern abstracts transaction lifecycle management (`BEGIN`, `COMMIT`, `ROLLBACK`) away from business logic. By encapsulating transactional operations within a closure (`ExecuteTx(ctx, func(tx TxScope) error)`), the pattern guarantees that:
1. Multi-repository operations execute within a single atomic database transaction.
2. Any returned error or runtime panic triggers an automatic, safe `ROLLBACK`.
3. Changes are committed atomically only when the entire closure completes successfully.

---

## Problem Statement (Failure scenarios without this pattern)
Managing raw SQL transactions manually within business services causes severe financial and state inconsistencies:
- **Inconsistent Partial Commits (Dangling Transactions)**: A financial transfer service deducts funds from Wallet A and then fails while crediting Wallet B. Without an atomic unit of work, Wallet A loses money while Wallet B receives nothing.
- **Leaked Uncommitted Transactions on Panics**: An unexpected `nil` pointer panic or early return in business code skips the `tx.Commit()` / `tx.Rollback()` line, leaving open transaction locks on tables until database server timeouts kill the session.
- **Polluting Domain Logic with `*sql.Tx`**: Passing raw database transaction pointers across domain services couples business logic directly to the database driver and makes mocking transactions difficult.

---

## Architectural Mechanism & Flow

```mermaid
flowchart TD
    A[Domain Service / TransferFunds] --> B["UnitOfWork.ExecuteTx(ctx, fn)"]
    
    subgraph Transaction Session Lifecycle
        B --> C[Begin Transaction / Instantiate TxScope]
        C --> D[Execute User Closure: fn(tx)]
        
        subgraph Operations inside TxScope
            D --> E1[tx.GetWallet Wallet A]
            D --> E2[tx.UpdateWallet Debit A]
            D --> E3[tx.UpdateWallet Credit B]
            D --> E4[tx.CreateLedgerEntry Audit Log]
        end
        
        D --> F{Closure Result: Success, Error, or Panic?}
        F -- Panic Detected --> G[Recover & Automatic Rollback: Discard Staged Changes]
        F -- Error Returned --> H[Automatic Rollback: Discard Staged Changes]
        F -- Success nil --> I[Atomic Commit: Apply All Staged Changes]
    end
    
    I --> J[Return Success to Caller]
    G --> K[Return Wrapped Panic Error]
    H --> L[Return Wrapped Domain Error]
```

### Component Architecture
1. **`TxScope` Interface**: Scopes operations available inside a transaction (`GetWallet`, `UpdateWallet`, `CreateLedgerEntry`).
2. **`UnitOfWork` Interface**: Declares `ExecuteTx(ctx context.Context, fn func(tx TxScope) error) error`.
3. **Panic-Safe Rollback Defer**: A deferred recovery handler intercepts panics, ensures rollback, and converts panics to clean Go errors.

---

## Production Best Practices & Pitfalls

### Best Practices
- **Use Closure-Based Transactions**: Enclosing transactions in `ExecuteTx(ctx, fn)` guarantees that rollbacks always occur on unexpected return paths, errors, or panics.
- **Recover Panics in Transaction Wrappers**: Always place a `recover()` block inside the deferred rollback handler to prevent unhandled panics from leaving open database transaction handles.
- **Keep Transactions Short**: Execute only fast database queries within the transaction closure. Avoid making slow external network calls (e.g. Stripe API, HTTP endpoints) while holding active database row locks.
- **Propagate Context Deadlines**: Ensure all queries executed on `TxScope` respect the outer transaction context deadline.

### Common Pitfalls
- **Making External HTTP Calls Inside Transactions**: Calling third-party APIs while holding database locks, causing connection pool exhaustion during external API latency spikes.
- **Nested Transactions Without Savepoints**: Calling `ExecuteTx` inside another `ExecuteTx` without savepoint support, causing confusing transaction commits or driver panics.
- **Passing `*sql.Tx` Across Package Boundaries**: Leaking concrete SQL transaction objects into clean architecture domain layers.

---

## Code Walkthrough & Usage

The implementation in `unit_of_work.go` demonstrates atomic transaction execution with automatic rollback on error:

```go
package main

import (
	"context"
	"errors"
	"log"

	"patterns/04_persistence"
)

func TransferFunds(ctx context.Context, uow persistence.UnitOfWork, fromID, toID string, amount int64) error {
	return uow.ExecuteTx(ctx, func(tx persistence.TxScope) error {
		// 1. Fetch source wallet
		fromWallet, err := tx.GetWallet(ctx, fromID)
		if err != nil {
			return err
		}
		if fromWallet.Balance < amount {
			return errors.New("insufficient funds")
		}

		// 2. Fetch destination wallet
		toWallet, err := tx.GetWallet(ctx, toID)
		if err != nil {
			return err
		}

		// 3. Update balances
		fromWallet.Balance -= amount
		toWallet.Balance += amount

		if err := tx.UpdateWallet(ctx, fromWallet); err != nil {
			return err
		}
		if err := tx.UpdateWallet(ctx, toWallet); err != nil {
			return err
		}

		// 4. Create immutable audit ledger log
		return tx.CreateLedgerEntry(ctx, &persistence.LedgerEntry{
			ID:        "ledg_01",
			WalletID:  fromID,
			Amount:    -amount,
			Reference: "Transfer to " + toID,
		})
	})
}

func main() {
	uow := persistence.NewInMemoryUnitOfWork()

	// Seed accounts
	uow.SeedWallet(&persistence.Wallet{ID: "wallet_alice", Balance: 1000})
	uow.SeedWallet(&persistence.Wallet{ID: "wallet_bob", Balance: 200})

	ctx := context.Background()

	// Execute atomic transfer of $300
	if err := TransferFunds(ctx, uow, "wallet_alice", "wallet_bob", 300); err != nil {
		log.Fatalf("Transfer failed: %v", err)
	}

	aliceBal, _ := uow.GetBalance("wallet_alice")
	bobBal, _ := uow.GetBalance("wallet_bob")
	log.Printf("Transfer successful! Alice Balance: $%d, Bob Balance: $%d, Ledger Count: %d",
		aliceBal, bobBal, uow.LedgerCount())
}
```
