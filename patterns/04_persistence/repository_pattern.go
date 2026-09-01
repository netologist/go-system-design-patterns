package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("entity not found")
	ErrTimeout  = errors.New("query timeout exceeded")
)

// Product represents a persisted catalog entity.
type Product struct {
	ID        string
	Name      string
	Price     int64
	CreatedAt time.Time
}

// ProductRepository defines persistence interface with context.Context on every call.
type ProductRepository interface {
	FindByID(ctx context.Context, id string) (*Product, error)
	Save(ctx context.Context, p *Product) error
	Delete(ctx context.Context, id string) error
}

// InMemoryProductRepository is a thread-safe implementation respecting context deadlines.
type InMemoryProductRepository struct {
	mu           sync.RWMutex
	store        map[string]*Product
	queryLatency time.Duration // Simulated query latency
}

func NewInMemoryProductRepository(simulatedLatency time.Duration) *InMemoryProductRepository {
	return &InMemoryProductRepository{
		store:        make(map[string]*Product),
		queryLatency: simulatedLatency,
	}
}

func (r *InMemoryProductRepository) FindByID(ctx context.Context, id string) (*Product, error) {
	if err := r.simulateDelay(ctx); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.store[id]
	if !ok {
		return nil, fmt.Errorf("%w: product with id %s", ErrNotFound, id)
	}

	// Defensive copy
	return &Product{
		ID:        p.ID,
		Name:      p.Name,
		Price:     p.Price,
		CreatedAt: p.CreatedAt,
	}, nil
}

func (r *InMemoryProductRepository) Save(ctx context.Context, p *Product) error {
	if p == nil || p.ID == "" {
		return errors.New("invalid product")
	}

	if err := r.simulateDelay(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.store[p.ID] = &Product{
		ID:        p.ID,
		Name:      p.Name,
		Price:     p.Price,
		CreatedAt: p.CreatedAt,
	}
	return nil
}

func (r *InMemoryProductRepository) Delete(ctx context.Context, id string) error {
	if err := r.simulateDelay(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.store[id]; !ok {
		return fmt.Errorf("%w: product with id %s", ErrNotFound, id)
	}

	delete(r.store, id)
	return nil
}

func (r *InMemoryProductRepository) simulateDelay(ctx context.Context) error {
	if r.queryLatency <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	select {
	case <-time.After(r.queryLatency):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
