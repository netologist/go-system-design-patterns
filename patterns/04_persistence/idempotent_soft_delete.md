# Idempotent Soft Delete & Upsert Pattern

## Overview & Definition
The **Idempotent Soft Delete & Upsert** pattern provides non-destructive record deletion and safe, idempotent insert/update operations.

Rather than executing a physical `DELETE FROM table WHERE id = ?` (which permanently drops data and can break foreign key constraints across historical ledgers), soft deletion marks the record as inactive using a timestamp (`deleted_at *time.Time`). 

Idempotency guarantees that:
1. Deleting a record multiple times produces the exact same outcome (success with no error).
2. Upserting the same payload repeatedly produces the identical persisted state without generating duplicate versions or error responses.
3. Updating a previously soft-deleted record can automatically un-delete (reactivate) it if business logic requires.

---

## Problem Statement (Failure scenarios without this pattern)
Hard deletion and non-idempotent mutations create severe operational hazards:
- **Accidental Permanent Data Loss**: Accidental user or administrator deletion permanently wipes rows, making disaster recovery impossible without full database point-in-time restores.
- **Broken Audit Trails & Foreign Keys**: Hard deleting a user breaks historical references in audit logs, invoice records, and payment ledgers that rely on foreign key relations.
- **Retry Storm Failures on Network Drops**: In microservice architectures, if an HTTP client does not receive the response for a `DELETE` or `POST` call due to a transient timeout, it retries the call. If the delete is not idempotent, the second request fails with `404 Not Found`, causing client errors.

---

## Architectural Mechanism & Flow

```mermaid
flowchart TD
    subgraph Idempotent Soft Delete
        A[SoftDelete(ctx, id)] --> B{Record Exists?}
        B -- No --> C[Return ErrNotFound]
        B -- Yes --> D{Is DeletedAt != nil?}
        D -- Yes: Already Deleted --> E[Idempotent Return: Success nil]
        D -- No: Active Record --> F[Set DeletedAt = NOW & Update Timestamps]
        F --> G[Return Success nil]
    end
    
    subgraph Query Routing
        H[FindActive Query] --> I[WHERE deleted_at IS NULL]
        J[FindIncludingDeleted Query] --> K[WHERE id = ? (Audit Mode)]
    end
```

### Key Components
1. **Entity Schema (`Document`)**: Contains `DeletedAt *time.Time` and `IsDeleted() bool`.
2. **`IdempotentUpsert`**: Checks existing values; if content matches and the record is active, it returns early as an idempotent no-op without bumping versions.
3. **`SoftDelete`**: Idempotently marks `DeletedAt = &now`. Multiple successive calls return `nil`.
4. **Scoped Accessors**:
   - `FindActive`: Ignores soft-deleted records (used by standard customer-facing APIs).
   - `FindIncludingDeleted`: Returns records regardless of deletion status (used by audit and compliance logs).

---

## Production Best Practices & Pitfalls

### Best Practices
- **Partial Unique Indexes in SQL**: When using unique constraints on soft-deletable tables (e.g., unique email), use partial indexes: `CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;` so previously deleted emails can be re-registered.
- **Always Use Nullable Timestamps**: Use `*time.Time` or `sql.NullTime` for `deleted_at` rather than boolean flags (`is_deleted bool`) to preserve forensic audit timelines.
- **Idempotency in REST APIs**: Map `SoftDelete` to `DELETE /documents/:id`, returning `204 No Content` or `200 OK` regardless of whether the document was deleted on this call or a prior retry.
- **Implement Automated Purge / Archival Jobs**: Schedule background jobs to physically archive or purge soft-deleted records older than regulatory retention windows (e.g. GDPR 30-day purge).

### Common Pitfalls
- **Accidental Inclusion in Aggregation Queries**: Writing `SELECT COUNT(*) FROM documents` without filtering `WHERE deleted_at IS NULL`, distorting metrics and reports.
- **Breaking Unique Constraints with Standard Indexes**: Creating a standard `UNIQUE(email)` index that prevents re-inserting an email after the original user was soft-deleted.
- **Cascading Soft Deletion Traps**: Forgetting to propagate soft-deletion status to child relationship entities, leaving orphaned records active.

---

## Code Walkthrough & Usage

The implementation in `idempotent_soft_delete.go` demonstrates idempotent upserts, soft deletion, and filtered retrieval:

```go
package main

import (
	"context"
	"errors"
	"log"

	"patterns/04_persistence"
)

func main() {
	store := persistence.NewDocumentStore()
	ctx := context.Background()

	// 1. Idempotent Upsert (Initial creation)
	doc, err := store.IdempotentUpsert(ctx, "doc_101", "Architecture Spec", "Go Backend Patterns")
	if err != nil {
		log.Fatalf("Upsert failed: %v", err)
	}
	log.Printf("Document Created: Version=%d, Title=%s", doc.Version, doc.Title)

	// 2. Repeating identical upsert (No-op idempotent return, version remains 1)
	docSame, _ := store.IdempotentUpsert(ctx, "doc_101", "Architecture Spec", "Go Backend Patterns")
	log.Printf("Identical Upsert Version: %d (No change)", docSame.Version)

	// 3. First Soft Delete
	if err := store.SoftDelete(ctx, "doc_101"); err != nil {
		log.Fatalf("Soft delete failed: %v", err)
	}
	log.Println("Document soft-deleted successfully")

	// 4. Repeated Soft Delete (Idempotent: returns nil without error)
	if err := store.SoftDelete(ctx, "doc_101"); err != nil {
		log.Fatalf("Second delete should not fail: %v", err)
	}
	log.Println("Repeated soft-delete succeeded idempotently")

	// 5. Active query returns ErrNotFound
	_, err = store.FindActive(ctx, "doc_101")
	if errors.Is(err, persistence.ErrNotFound) {
		log.Println("FindActive correctly treats soft-deleted document as not found")
	}

	// 6. Admin query retrieves soft-deleted document for audit
	auditDoc, err := store.FindIncludingDeleted(ctx, "doc_101")
	if err != nil {
		log.Fatalf("Audit find failed: %v", err)
	}
	log.Printf("Audit found soft-deleted doc: DeletedAt=%v", auditDoc.DeletedAt)
}
```
