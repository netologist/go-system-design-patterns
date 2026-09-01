package messaging_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	messaging "system-design-patterns/patterns/15_messaging"
)

func TestGracefulConsumer_DrainsInFlightMessages(t *testing.T) {
	consumer := messaging.NewGracefulConsumer(context.Background(), 2, 10)

	var completedCount atomic.Int32
	totalTasks := 6

	for range totalTasks {
		_ = consumer.Enqueue(func(ctx context.Context) error {
			time.Sleep(15 * time.Millisecond)
			completedCount.Add(1)
			return nil
		})
	}

	// Trigger graceful shutdown
	err := consumer.Shutdown(500 * time.Millisecond)
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if completedCount.Load() != int32(totalTasks) {
		t.Errorf("expected all %d in-flight tasks drained, got: %d", totalTasks, completedCount.Load())
	}
}
