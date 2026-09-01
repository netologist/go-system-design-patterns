package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DeadLetterEntry represents a poison message routed to the DLQ.
type DeadLetterEntry struct {
	Message   Message
	LastError string
	Attempts  int
	FailedAt  time.Time
}

// DLQManager handles message processing retries and isolates poison messages into DLQ.
type DLQManager struct {
	mu          sync.Mutex
	maxAttempts int
	dlqStore    []DeadLetterEntry
	attempts    map[string]int
}

func NewDLQManager(maxAttempts int) *DLQManager {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	return &DLQManager{
		maxAttempts: maxAttempts,
		dlqStore:    make([]DeadLetterEntry, 0),
		attempts:    make(map[string]int),
	}
}

// ProcessWithDLQ attempts to process msg with handler. If it fails maxAttempts times, it is routed to DLQ.
func (d *DLQManager) ProcessWithDLQ(
	ctx context.Context,
	msg Message,
	handler func(ctx context.Context, m Message) error,
) error {
	d.mu.Lock()
	d.attempts[msg.ID]++
	currAttempts := d.attempts[msg.ID]
	d.mu.Unlock()

	err := handler(ctx, msg)
	if err == nil {
		d.mu.Lock()
		delete(d.attempts, msg.ID)
		d.mu.Unlock()
		return nil
	}

	if currAttempts >= d.maxAttempts {
		// Poison message detected -> Send to DLQ
		d.mu.Lock()
		defer d.mu.Unlock()

		d.dlqStore = append(d.dlqStore, DeadLetterEntry{
			Message:   msg,
			LastError: err.Error(),
			Attempts:  currAttempts,
			FailedAt:  time.Now(),
		})
		delete(d.attempts, msg.ID)

		return fmt.Errorf("message %s exceeded max retries (%d) and was routed to DLQ: %w", msg.ID, d.maxAttempts, err)
	}

	return fmt.Errorf("processing attempt %d/%d failed: %w", currAttempts, d.maxAttempts, err)
}

// DLQEntries returns a snapshot of isolated dead letter messages.
func (d *DLQManager) DLQEntries() []DeadLetterEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	res := make([]DeadLetterEntry, len(d.dlqStore))
	copy(res, d.dlqStore)
	return res
}
