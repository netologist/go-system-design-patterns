package messaging_test

import (
	"context"
	"errors"
	"testing"
	"time"

	messaging "system-design-patterns/patterns/15_messaging"
)

func TestDLQManager_RoutesPoisonMessage(t *testing.T) {
	dlq := messaging.NewDLQManager(3)
	ctx := context.Background()

	poisonMsg := messaging.Message{
		ID:        "poison-msg-1",
		Payload:   "invalid_corrupt_data",
		Timestamp: time.Now(),
	}

	alwaysFailingHandler := func(c context.Context, m messaging.Message) error {
		return errors.New("unrecoverable parsing syntax error")
	}

	// Attempt 1 -> Fail
	_ = dlq.ProcessWithDLQ(ctx, poisonMsg, alwaysFailingHandler)
	if len(dlq.DLQEntries()) != 0 {
		t.Errorf("message should not be in DLQ after attempt 1")
	}

	// Attempt 2 -> Fail
	_ = dlq.ProcessWithDLQ(ctx, poisonMsg, alwaysFailingHandler)
	if len(dlq.DLQEntries()) != 0 {
		t.Errorf("message should not be in DLQ after attempt 2")
	}

	// Attempt 3 -> Reaches maxAttempts=3, moves to DLQ
	err := dlq.ProcessWithDLQ(ctx, poisonMsg, alwaysFailingHandler)
	if err == nil {
		t.Fatal("expected DLQ routing error on max attempts")
	}

	entries := dlq.DLQEntries()
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 entry in DLQ, got: %d", len(entries))
	}

	if entries[0].Message.ID != "poison-msg-1" || entries[0].Attempts != 3 {
		t.Errorf("DLQ entry mismatch: %+v", entries[0])
	}
}
