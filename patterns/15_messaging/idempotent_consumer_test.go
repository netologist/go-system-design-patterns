package messaging_test

import (
	"context"
	"testing"
	"time"

	messaging "system-design-patterns/patterns/15_messaging"
)

func TestIdempotentConsumer_Deduplication(t *testing.T) {
	consumer := messaging.NewIdempotentConsumer()
	ctx := context.Background()

	msg := messaging.Message{
		ID:        "evt-ord-1234",
		Payload:   "OrderCreated",
		Timestamp: time.Now(),
	}

	executions := 0
	handler := func(c context.Context, m messaging.Message) error {
		executions++
		return nil
	}

	// 1st delivery
	err := consumer.ProcessMessage(ctx, msg, handler)
	if err != nil || executions != 1 {
		t.Fatalf("expected 1st delivery to execute handler, got exec=%d err=%v", executions, err)
	}

	// 2nd delivery (duplicate message)
	err = consumer.ProcessMessage(ctx, msg, handler)
	if err != nil {
		t.Fatalf("expected duplicate delivery to succeed idempotently, got: %v", err)
	}
	if executions != 1 {
		t.Errorf("duplicate message was re-executed! executions: %d", executions)
	}
}
