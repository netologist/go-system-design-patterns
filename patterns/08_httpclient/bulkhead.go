package httpclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrBulkheadFull = errors.New("bulkhead capacity exceeded for partition")

// PartitionSemaphore manages concurrency for an isolated workload.
type PartitionSemaphore struct {
	tokens chan struct{}
}

func newPartitionSemaphore(maxConcurrency int) *PartitionSemaphore {
	tokens := make(chan struct{}, maxConcurrency)
	for range maxConcurrency {
		tokens <- struct{}{}
	}
	return &PartitionSemaphore{tokens: tokens}
}

// BulkheadRegistry coordinates isolated concurrency pools per downstream service / workload.
type BulkheadRegistry struct {
	mu         sync.RWMutex
	partitions map[string]*PartitionSemaphore
}

func NewBulkheadRegistry() *BulkheadRegistry {
	return &BulkheadRegistry{
		partitions: make(map[string]*PartitionSemaphore),
	}
}

// RegisterPartition defines maximum concurrency for a named workload.
func (b *BulkheadRegistry) RegisterPartition(name string, maxConcurrency int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	b.partitions[name] = newPartitionSemaphore(maxConcurrency)
}

// Execute runs fn within the specified bulkhead partition, rejecting if capacity is saturated.
func (b *BulkheadRegistry) Execute(ctx context.Context, partitionName string, fn func(ctx context.Context) error) error {
	b.mu.RLock()
	sem, ok := b.partitions[partitionName]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown bulkhead partition '%s'", partitionName)
	}

	// Try to acquire slot non-blocking or with context
	select {
	case <-sem.tokens:
		defer func() {
			sem.tokens <- struct{}{}
		}()
		return fn(ctx)
	default:
		// Saturated -> Fail fast or wait on context
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %w", ErrBulkheadFull, ctx.Err())
		case <-sem.tokens:
			defer func() {
				sem.tokens <- struct{}{}
			}()
			return fn(ctx)
		default:
			return fmt.Errorf("%w for partition '%s'", ErrBulkheadFull, partitionName)
		}
	}
}
