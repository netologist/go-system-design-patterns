package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrOptimisticLockConflict = errors.New("optimistic lock conflict: record has been modified by another transaction")

// InventoryItem represents a product stock with optimistic locking version.
type InventoryItem struct {
	ID        string
	SKU       string
	Stock     int
	Version   int64 // Incremented on every update
	UpdatedAt time.Time
}

// OptimisticInventoryStore manages version-controlled inventory.
type OptimisticInventoryStore struct {
	mu    sync.RWMutex
	items map[string]*InventoryItem
}

func NewOptimisticInventoryStore() *OptimisticInventoryStore {
	return &OptimisticInventoryStore{
		items: make(map[string]*InventoryItem),
	}
}

func (s *OptimisticInventoryStore) Create(item *InventoryItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item.Version = 1
	item.UpdatedAt = time.Now()
	s.items[item.ID] = &InventoryItem{
		ID:        item.ID,
		SKU:       item.SKU,
		Stock:     item.Stock,
		Version:   item.Version,
		UpdatedAt: item.UpdatedAt,
	}
}

func (s *OptimisticInventoryStore) Get(id string) (*InventoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &InventoryItem{
		ID:        item.ID,
		SKU:       item.SKU,
		Stock:     item.Stock,
		Version:   item.Version,
		UpdatedAt: item.UpdatedAt,
	}, nil
}

// UpdateStock executes conditional update matching the exact version.
func (s *OptimisticInventoryStore) UpdateStock(ctx context.Context, id string, expectedVersion int64, newStock int) (*InventoryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}

	if item.Version != expectedVersion {
		return nil, fmt.Errorf("%w: expected version %d but current is %d",
			ErrOptimisticLockConflict, expectedVersion, item.Version)
	}

	item.Stock = newStock
	item.Version++
	item.UpdatedAt = time.Now()

	return &InventoryItem{
		ID:        item.ID,
		SKU:       item.SKU,
		Stock:     item.Stock,
		Version:   item.Version,
		UpdatedAt: item.UpdatedAt,
	}, nil
}
