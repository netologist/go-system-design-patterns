package messaging

import (
	"context"
	"sync"
	"time"
)

// Message represents an event delivered with at-least-once semantics.
type Message struct {
	ID        string
	Payload   string
	Timestamp time.Time
}

// IdempotentConsumer processes messages only once, skipping duplicate deliveries.
type IdempotentConsumer struct {
	mu           sync.RWMutex
	processedIDs map[string]time.Time
}

func NewIdempotentConsumer() *IdempotentConsumer {
	return &IdempotentConsumer{
		processedIDs: make(map[string]time.Time),
	}
}

// ProcessMessage executes handler only if msg.ID has not been processed before.
func (c *IdempotentConsumer) ProcessMessage(ctx context.Context, msg Message, handler func(ctx context.Context, m Message) error) error {
	c.mu.Lock()
	if _, seen := c.processedIDs[msg.ID]; seen {
		c.mu.Unlock()
		// Duplicate delivery detected: skip processing idempotently
		return nil
	}
	c.mu.Unlock()

	// Execute business logic
	if err := handler(ctx, msg); err != nil {
		return err
	}

	// Mark as processed
	c.mu.Lock()
	c.processedIDs[msg.ID] = time.Now()
	c.mu.Unlock()

	return nil
}

// IsProcessed checks if a message ID has been handled.
func (c *IdempotentConsumer) IsProcessed(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, seen := c.processedIDs[id]
	return seen
}
