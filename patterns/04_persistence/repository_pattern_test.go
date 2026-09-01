package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	persistence "system-design-patterns/patterns/04_persistence"
)

func TestRepository_SaveAndFind(t *testing.T) {
	repo := persistence.NewInMemoryProductRepository(0)
	ctx := context.Background()

	p := &persistence.Product{
		ID:        "prod-1",
		Name:      "Mechanical Keyboard",
		Price:     12000,
		CreatedAt: time.Now(),
	}

	if err := repo.Save(ctx, p); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	found, err := repo.FindByID(ctx, "prod-1")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	if found.Name != "Mechanical Keyboard" || found.Price != 12000 {
		t.Errorf("found data mismatch: %+v", found)
	}
}

func TestRepository_NotFound(t *testing.T) {
	repo := persistence.NewInMemoryProductRepository(0)
	_, err := repo.FindByID(context.Background(), "non-existent")
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestRepository_ContextTimeout(t *testing.T) {
	// Simulate a 100ms slow DB query
	repo := persistence.NewInMemoryProductRepository(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := repo.FindByID(ctx, "prod-1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
	}
}
