package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type OutboxStatus string

const (
	StatusPending   OutboxStatus = "PENDING"
	StatusPublished OutboxStatus = "PUBLISHED"
)

// OutboxEvent represents an event record stored in the transactional outbox table.
type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       string
	Status        OutboxStatus
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

// TransactionalOutboxStore simulates atomic database persistence of domain records and outbox events.
type TransactionalOutboxStore struct {
	mu           sync.Mutex
	outboxEvents map[string]*OutboxEvent
}

func NewTransactionalOutboxStore() *TransactionalOutboxStore {
	return &TransactionalOutboxStore{
		outboxEvents: make(map[string]*OutboxEvent),
	}
}

// SaveTx atomically commits business entity and outbox event.
func (s *TransactionalOutboxStore) SaveTx(ctx context.Context, event *OutboxEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event.Status = StatusPending
	event.CreatedAt = time.Now().UTC()
	s.outboxEvents[event.ID] = event
	return nil
}

// GetPendingEvents retrieves unpublished outbox events.
func (s *TransactionalOutboxStore) GetPendingEvents(ctx context.Context, limit int) []*OutboxEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pending []*OutboxEvent
	for _, e := range s.outboxEvents {
		if e.Status == StatusPending {
			pending = append(pending, e)
			if len(pending) >= limit {
				break
			}
		}
	}
	return pending
}

// MarkPublished updates status to PUBLISHED.
func (s *TransactionalOutboxStore) MarkPublished(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.outboxEvents[id]
	if !ok {
		return fmt.Errorf("outbox event %s not found", id)
	}

	now := time.Now().UTC()
	e.Status = StatusPublished
	e.PublishedAt = &now
	return nil
}

// OutboxRelayPublisher background relay publisher to Kafka / RabbitMQ.
type OutboxRelayPublisher struct {
	store     *TransactionalOutboxStore
	publishFn func(ctx context.Context, event *OutboxEvent) error
}

func NewOutboxRelayPublisher(store *TransactionalOutboxStore, publishFn func(ctx context.Context, event *OutboxEvent) error) *OutboxRelayPublisher {
	return &OutboxRelayPublisher{
		store:     store,
		publishFn: publishFn,
	}
}

func (p *OutboxRelayPublisher) Flush(ctx context.Context) (int, error) {
	events := p.store.GetPendingEvents(ctx, 50)
	publishedCount := 0

	for _, e := range events {
		if err := p.publishFn(ctx, e); err != nil {
			return publishedCount, err
		}
		_ = p.store.MarkPublished(ctx, e.ID)
		publishedCount++
	}

	return publishedCount, nil
}
