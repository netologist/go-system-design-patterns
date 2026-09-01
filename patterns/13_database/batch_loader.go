package database

import (
	"context"
	"errors"
	"sync"
	"time"
)

type loadRequest[K comparable, V any] struct {
	key    K
	result chan V
	err    chan error
}

// BatchLoader aggregates individual key lookups within a small window into a single batch query, eliminating N+1 queries.
type BatchLoader[K comparable, V any] struct {
	mu           sync.Mutex
	batchFn      func(ctx context.Context, keys []K) (map[K]V, error)
	waitDuration time.Duration
	maxBatchSize int
	pending      []loadRequest[K, V]
	timer        *time.Timer
}

// NewBatchLoader creates a batch loader with batching window and maximum size.
func NewBatchLoader[K comparable, V any](
	waitDuration time.Duration,
	maxBatchSize int,
	batchFn func(ctx context.Context, keys []K) (map[K]V, error),
) *BatchLoader[K, V] {
	if waitDuration <= 0 {
		waitDuration = 2 * time.Millisecond
	}
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}

	return &BatchLoader[K, V]{
		batchFn:      batchFn,
		waitDuration: waitDuration,
		maxBatchSize: maxBatchSize,
	}
}

// Load requests a single item by key, participating in the current batch.
func (b *BatchLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
	req := loadRequest[K, V]{
		key:    key,
		result: make(chan V, 1),
		err:    make(chan error, 1),
	}

	b.mu.Lock()
	b.pending = append(b.pending, req)

	if len(b.pending) >= b.maxBatchSize {
		// Trigger immediate flush
		if b.timer != nil {
			b.timer.Stop()
			b.timer = nil
		}
		pendingToFlush := b.pending
		b.pending = nil
		b.mu.Unlock()
		go b.flush(ctx, pendingToFlush)
	} else {
		if b.timer == nil {
			b.timer = time.AfterFunc(b.waitDuration, func() {
				b.mu.Lock()
				pendingToFlush := b.pending
				b.pending = nil
				b.timer = nil
				b.mu.Unlock()
				if len(pendingToFlush) > 0 {
					b.flush(ctx, pendingToFlush)
				}
			})
		}
		b.mu.Unlock()
	}

	select {
	case <-ctx.Done():
		var zero V
		return zero, ctx.Err()
	case err := <-req.err:
		var zero V
		return zero, err
	case val := <-req.result:
		return val, nil
	}
}

func (b *BatchLoader[K, V]) flush(ctx context.Context, requests []loadRequest[K, V]) {
	keysMap := make(map[K]struct{})
	var uniqueKeys []K

	for _, req := range requests {
		if _, exists := keysMap[req.key]; !exists {
			keysMap[req.key] = struct{}{}
			uniqueKeys = append(uniqueKeys, req.key)
		}
	}

	results, err := b.batchFn(ctx, uniqueKeys)
	if err != nil {
		for _, req := range requests {
			req.err <- err
		}
		return
	}

	for _, req := range requests {
		if val, ok := results[req.key]; ok {
			req.result <- val
		} else {
			req.err <- errors.New("key not found in batch results")
		}
	}
}
