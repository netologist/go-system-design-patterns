# Property-Based & Invariant Verification Pattern

## 1. Overview & Concept
Property-based testing verifies that core system **invariants** (rules that must *always* remain true regardless of state) hold across arbitrary, randomly generated sequences of operations, rather than testing hand-crafted individual test inputs.

Example Invariant: *In a banking ledger, the total sum of money across all accounts must remain strictly identical before and after any number of random transfers.*

## 2. Production Problem & Failure Modes
1. **Edge-Case Blind Spots**: Example-based unit tests only check scenarios developers actively anticipated (e.g. transfer $50 from A to B). Complex race conditions, rounding bugs, or negative balances under random interleavings are missed.
2. **Hidden Invariant Violations**: Multi-step workflows can leave data in corrupt intermediate states that example tests don't reveal.
3. **Data Integrity Bugs**: Financial, inventory, and ledger calculations require mathematical conservation laws that unit tests fail to prove globally.

## 3. Architecture & Mechanism

```text
Initial System State (Sum = $400,000)
             |
             v
Loop N times (e.g. 500 random operations):
   - Pick random source account
   - Pick random destination account
   - Generate random amount
   - Execute Transfer()
   - ASSERT: Sum(All Accounts) == $400,000 (INVARIANT CHECK)
```

## 4. Production Hardening & Trade-offs
- **State Invariants**: Formulate domain invariants explicitly (e.g. `Account.Balance >= 0`, `TotalStock == Sum(WarehouseStocks)`).
- **Concurrency Invariants**: Combine invariant checks with concurrent goroutine execution to detect race condition corruptions.
- **Shrinking**: When an invariant fails, isolate the minimal sequence of steps causing the violation for fast reproduction.

## 5. Code Walkthrough & Usage
See `property_invariant.go` and `property_invariant_test.go`:
- `BankingLedger`: Enforces atomic transfers and computes `TotalSystemBalance()`.
- Test executes 200 random transfers and verifies conservation invariant after every single step.
