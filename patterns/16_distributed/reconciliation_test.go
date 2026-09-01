package distributed_test

import (
	"context"
	"testing"

	distributed "system-design-patterns/patterns/16_distributed"
)

func TestReconciliationEngine_RepairsDrift(t *testing.T) {
	ctx := context.Background()

	source := distributed.NewMemoryReconciliationStore()
	target := distributed.NewMemoryReconciliationStore()

	// Source has:
	// - rec-1 (v2, latest)
	// - rec-2 (v1, new)
	_ = source.Upsert(ctx, distributed.Record{ID: "rec-1", Version: 2, Data: "Updated Data"})
	_ = source.Upsert(ctx, distributed.Record{ID: "rec-2", Version: 1, Data: "New Data"})

	// Target has:
	// - rec-1 (v1, outdated)
	// - rec-orphan (v1, deleted from source)
	_ = target.Upsert(ctx, distributed.Record{ID: "rec-1", Version: 1, Data: "Stale Data"})
	_ = target.Upsert(ctx, distributed.Record{ID: "rec-orphan", Version: 1, Data: "Orphaned Data"})

	engine := distributed.NewReconciliationEngine(source, target)
	report, err := engine.Reconcile(ctx)
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	if len(report.MissingInTarget) != 1 || report.MissingInTarget[0] != "rec-2" {
		t.Errorf("expected rec-2 identified as missing, got: %v", report.MissingInTarget)
	}
	if len(report.MismatchInTarget) != 1 || report.MismatchInTarget[0] != "rec-1" {
		t.Errorf("expected rec-1 identified as mismatch, got: %v", report.MismatchInTarget)
	}
	if len(report.OrphansInTarget) != 1 || report.OrphansInTarget[0] != "rec-orphan" {
		t.Errorf("expected rec-orphan identified as orphan, got: %v", report.OrphansInTarget)
	}
	if report.RepairedCount != 3 {
		t.Errorf("expected 3 repaired items, got: %d", report.RepairedCount)
	}
}
