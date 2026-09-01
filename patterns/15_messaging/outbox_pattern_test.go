package messaging_test

import (
	"context"
	"testing"

	messaging "system-design-patterns/patterns/15_messaging"
)

func TestTransactionalOutbox_SaveAndFlush(t *testing.T) {
	store := messaging.NewTransactionalOutboxStore()
	ctx := context.Background()

	// 1. Transaction commits business state + outbox event
	event := &messaging.OutboxEvent{
		ID:            "evt-100",
		AggregateType: "Order",
		AggregateID:   "ord-99",
		EventType:     "OrderPaid",
		Payload:       `{"amount": 9900}`,
	}

	err := store.SaveTx(ctx, event)
	if err != nil {
		t.Fatalf("save outbox failed: %v", err)
	}

	// 2. Relay publishes to message broker
	publishedEvents := 0
	publisher := messaging.NewOutboxRelayPublisher(store, func(c context.Context, e *messaging.OutboxEvent) error {
		publishedEvents++
		return nil
	})

	count, err := publisher.Flush(ctx)
	if err != nil || count != 1 {
		t.Fatalf("expected 1 event flushed, got %d (err: %v)", count, err)
	}

	// 3. Verify no more pending events
	pending := store.GetPendingEvents(ctx, 10)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending events after flush, got: %d", len(pending))
	}
}
