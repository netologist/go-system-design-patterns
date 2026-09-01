package messaging_test

import (
	"context"
	"errors"
	"testing"

	messaging "system-design-patterns/patterns/15_messaging"
)

func TestInboxStore_DeduplicationAndLocking(t *testing.T) {
	inbox := messaging.NewInboxStore()
	ctx := context.Background()

	eventID := "evt-kafka-12345"

	// 1. First worker starts processing
	err := inbox.TryStartProcessing(ctx, eventID)
	if err != nil {
		t.Fatalf("expected start processing to succeed, got: %v", err)
	}

	// 2. Concurrent worker attempts same event -> Locked
	err = inbox.TryStartProcessing(ctx, eventID)
	if !errors.Is(err, messaging.ErrCurrentlyLocked) {
		t.Errorf("expected ErrCurrentlyLocked, got: %v", err)
	}

	// 3. First worker completes
	err = inbox.MarkCompleted(ctx, eventID)
	if err != nil {
		t.Fatalf("mark completed failed: %v", err)
	}

	// 4. Duplicate re-delivery attempt -> Already Processed
	err = inbox.TryStartProcessing(ctx, eventID)
	if !errors.Is(err, messaging.ErrAlreadyProcessed) {
		t.Errorf("expected ErrAlreadyProcessed on redelivery, got: %v", err)
	}
}
