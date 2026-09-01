package persistence_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	persistence "system-design-patterns/patterns/04_persistence"
)

func TestOptimisticLocking_ConcurrentUpdates(t *testing.T) {
	store := persistence.NewOptimisticInventoryStore()
	store.Create(&persistence.InventoryItem{
		ID:    "inv-1",
		SKU:   "LAPTOP-01",
		Stock: 100,
	})

	// User A reads version 1
	userAItem, _ := store.Get("inv-1")
	// User B reads version 1
	userBItem, _ := store.Get("inv-1")

	// User A updates stock to 90
	updatedA, err := store.UpdateStock(context.Background(), "inv-1", userAItem.Version, 90)
	if err != nil {
		t.Fatalf("User A update failed: %v", err)
	}
	if updatedA.Version != 2 {
		t.Errorf("expected version 2 after first update, got: %d", updatedA.Version)
	}

	// User B tries to update using stale version 1 -> Must Fail
	_, err = store.UpdateStock(context.Background(), "inv-1", userBItem.Version, 80)
	if err == nil {
		t.Fatal("expected optimistic lock conflict for User B, got nil")
	}
	if !errors.Is(err, persistence.ErrOptimisticLockConflict) {
		t.Errorf("expected ErrOptimisticLockConflict, got: %v", err)
	}
}

func TestOptimisticLocking_RaceResolution(t *testing.T) {
	store := persistence.NewOptimisticInventoryStore()
	store.Create(&persistence.InventoryItem{
		ID:    "inv-race",
		SKU:   "PHONE-01",
		Stock: 50,
	})

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// 10 concurrent requests all reading version 1 and trying to decrement
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.UpdateStock(context.Background(), "inv-race", 1, 40)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Exactly one must succeed
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful update at version 1, got: %d", successCount)
	}
}
