# Continuous State Reconciliation (Convergence) Pattern

## Overview & Definition
The **Reconciliation Pattern** (also known as the **Reconciliation Loop** or **Continuous State Convergence Pattern**, popularized by the Kubernetes Control Plane architecture) continuously drives an actual observed system state toward a desired declarative state.

In distributed backend architectures, components synchronize state asynchronously via event streams, webhooks, or cache invalidation events. However, distributed networks are inherently unreliable: message broker network partitions, dropped webhooks, worker crashes, or out-of-order deliveries inevitably cause **data drift** between the Primary Source of Truth (e.g., authoritative database) and downstream secondary stores (e.g., Elasticsearch search indexes, Redis caches, read replicas, or third-party partner systems).

The Reconciliation Engine runs periodic or event-driven audit loops that:
1. Scan and compare entities across source and target stores.
2. Classify drift into **Missing**, **Mismatched (Version Lag)**, or **Orphaned (Deleted in Source)** categories.
3. Automatically execute idempotent repair actions (`Upsert` / `Delete`) to restore 100% convergence.

---

## Problem Statement (Failure Scenarios Without This Pattern)

### 1. The "Ghost Record" and Silent Search Drift Hazard
Consider an e-commerce catalog where product updates are synchronized to Elasticsearch via Kafka events:
- A network partition occurs while publishing `ProductDeletedEvent(ID="prod_55")`.
- The product is deleted from the PostgreSQL database, but Elasticsearch never receives the deletion event.
- **Failure**: Customers search for the product on the website, find it, click buy, and receive an error at checkout because the item does not exist. Without an active reconciliation loop, this silent drift persists forever.

### 2. Financial Ledger Discrepancies
In fintech and billing engines, webhook delivery failures from payment processors (e.g., Stripe) can cause invoice statuses in the local database to remain "Pending" even after funds were settled externally, leading to delayed payouts and inaccurate accounting.

---

## Architectural Mechanism & Flow

```mermaid
flowchart TD
    Trigger([Periodic Cron / Drift Event Trigger]) --> ReconcileLoop[ReconciliationEngine.Reconcile]
    
    subgraph Comparison [Audit & Drift Classification]
        ReconcileLoop --> FetchSrc[Fetch All from Source of Truth]
        ReconcileLoop --> FetchTgt[Fetch All from Target System]
        FetchSrc & FetchTgt --> DiffEngine{Compare IDs & Versions}
    end
    
    subgraph SelfHealing [Automatic Self-Healing Actions]
        DiffEngine -- Missing in Target --> UpsertMissing[Target.Upsert(Missing)]
        DiffEngine -- Version Mismatch in Target --> UpsertMismatch[Target.Upsert(Updated)]
        DiffEngine -- Orphan in Target (Deleted in Source) --> DeleteOrphan[Target.Delete(Orphan)]
    end
    
    UpsertMissing & UpsertMismatch & DeleteOrphan --> Report[Generate ReconciliationReport<br/>- MissingInTarget<br/>- MismatchInTarget<br/>- OrphansInTarget<br/>- RepairedCount]
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **Version / Timestamp Comparison**: Include a monotonic version number (`Version int64`) or modification timestamp in records so the engine can verify which entity is strictly newer.
- **Chunked / Cursor-Based Comparison**: For tables with millions of records, do not load all rows into memory at once; use cursor pagination, partition hashing, or Merkle trees to compare state in lightweight batches.
- **Safe Dry-Run Modes**: Provide configuration flags allowing operators to run reconciliation in "Dry-Run Audit" mode, logging discrepancies without automatically applying destructive deletions.
- **Emit Metrics on Drift**: Instrument reconciliation runs with Prometheus gauges (`reconciliation_drift_detected_total`, `reconciliation_repaired_total`) to detect upstream pipeline regressions early.

### Pitfalls to Avoid
- **Overwriting Newer State with Stale Data**: Ensure the source of truth is authoritatively determined so that a slower replica does not overwrite newer primary data.
- **High-Frequency Aggressive Polling**: Space out full reconciliation cycles (e.g., hourly or daily off-peak) to prevent excessive read loads on production primary databases.

---

## Code Walkthrough & Usage

In `patterns/16_distributed/reconciliation.go`, `ReconciliationEngine` compares datasets and repairs discrepancies:

```go
type Record struct {
    ID      string
    Version int64
    Data    string
}

type ReconciliationReport struct {
    MissingInTarget  []string
    MismatchInTarget []string
    OrphansInTarget  []string
    RepairedCount    int
}

type ReconciliationEngine struct {
    source ReconciliationStore
    target ReconciliationStore
}
```

### Self-Healing Reconciliation Implementation
```go
func (e *ReconciliationEngine) Reconcile(ctx context.Context) (*ReconciliationReport, error) {
    sourceRecords, err := e.source.GetAll(ctx)
    if err != nil {
        return nil, err
    }

    targetRecords, err := e.target.GetAll(ctx)
    if err != nil {
        return nil, err
    }

    report := &ReconciliationReport{}

    // 1. Detect and repair missing or outdated records in target
    for id, srcRec := range sourceRecords {
        tgtRec, exists := targetRecords[id]
        if !exists {
            report.MissingInTarget = append(report.MissingInTarget, id)
            _ = e.target.Upsert(ctx, srcRec)
            report.RepairedCount++
        } else if tgtRec.Version < srcRec.Version || tgtRec.Data != srcRec.Data {
            report.MismatchInTarget = append(report.MismatchInTarget, id)
            _ = e.target.Upsert(ctx, srcRec)
            report.RepairedCount++
        }
    }

    // 2. Detect and purge orphaned records in target (deleted from source)
    for id := range targetRecords {
        if _, exists := sourceRecords[id]; !exists {
            report.OrphansInTarget = append(report.OrphansInTarget, id)
            _ = e.target.Delete(ctx, id)
            report.RepairedCount++
        }
    }

    return report, nil
}
```
