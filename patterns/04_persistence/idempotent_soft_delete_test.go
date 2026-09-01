package persistence_test

import (
	"context"
	"errors"
	"testing"

	persistence "system-design-patterns/patterns/04_persistence"
)

func TestDocumentStore_IdempotentUpsert(t *testing.T) {
	store := persistence.NewDocumentStore()
	ctx := context.Background()

	// Initial insert
	doc1, err := store.IdempotentUpsert(ctx, "doc-1", "Architecture Spec", "Body Content")
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if doc1.Version != 1 {
		t.Errorf("expected version 1, got: %d", doc1.Version)
	}

	// Repeated identical upsert (no changes)
	doc2, err := store.IdempotentUpsert(ctx, "doc-1", "Architecture Spec", "Body Content")
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if doc2.Version != 1 {
		t.Errorf("expected version to remain 1 on identical upsert, got: %d", doc2.Version)
	}

	// Update with new content
	doc3, err := store.IdempotentUpsert(ctx, "doc-1", "Architecture Spec v2", "Updated Body")
	if err != nil {
		t.Fatalf("update upsert failed: %v", err)
	}
	if doc3.Version != 2 {
		t.Errorf("expected version 2 after modification, got: %d", doc3.Version)
	}
}

func TestDocumentStore_SoftDeleteAndIdempotency(t *testing.T) {
	store := persistence.NewDocumentStore()
	ctx := context.Background()

	_, _ = store.IdempotentUpsert(ctx, "doc-2", "Secret Document", "Top Secret")

	// Soft delete
	if err := store.SoftDelete(ctx, "doc-2"); err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}

	// FindActive should fail
	_, err := store.FindActive(ctx, "doc-2")
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Errorf("expected ErrNotFound for soft-deleted doc, got: %v", err)
	}

	// FindIncludingDeleted should succeed
	doc, err := store.FindIncludingDeleted(ctx, "doc-2")
	if err != nil || !doc.IsDeleted() {
		t.Errorf("expected doc to be found and marked as deleted")
	}

	// Repeated soft delete must succeed idempotently
	if err := store.SoftDelete(ctx, "doc-2"); err != nil {
		t.Fatalf("repeated soft delete failed: %v", err)
	}
}
