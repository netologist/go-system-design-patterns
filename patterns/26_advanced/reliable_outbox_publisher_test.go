package advanced_test

import (
	"context"
	"sync"
	"testing"
	"time"

	advanced "system-design-patterns/patterns/26_advanced"
	messaging "system-design-patterns/patterns/15_messaging"
)

type mockBrokerProducer struct {
	mu        sync.Mutex
	published map[string][]string
}

func (m *mockBrokerProducer) Publish(ctx context.Context, topic string, payload string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.published == nil {
		m.published = make(map[string][]string)
	}
	m.published[topic] = append(m.published[topic], payload)
	return nil
}

func TestReliableOutboxPublisher_Flush(t *testing.T) {
	outboxStore := messaging.NewTransactionalOutboxStore()
	broker := &mockBrokerProducer{}
	ctx := context.Background()

	// 1. Transaction commits order and outbox event
	_ = outboxStore.SaveTx(ctx, &messaging.OutboxEvent{
		ID:        "evt-1",
		EventType: "order.created",
		Payload:   `{"order_id":"123"}`,
	})
	_ = outboxStore.SaveTx(ctx, &messaging.OutboxEvent{
		ID:        "evt-2",
		EventType: "inventory.reserved",
		Payload:   `{"item_id":"xyz"}`,
	})

	publisher := advanced.NewReliableOutboxPublisher(ctx, outboxStore, broker, 10*time.Millisecond)

	// 2. Flush once
	err := publisher.FlushOnce(ctx)
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	broker.mu.Lock()
	totalPublished := len(broker.published["order.created"]) + len(broker.published["inventory.reserved"])
	broker.mu.Unlock()

	if totalPublished != 2 {
		t.Errorf("expected 2 published messages to broker, got: %d", totalPublished)
	}

	// Verify outbox store has 0 pending
	pending := outboxStore.GetPendingEvents(ctx, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending events in outbox after flush, got: %d", len(pending))
	}
}
