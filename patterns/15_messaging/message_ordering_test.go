package messaging_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	messaging "system-design-patterns/patterns/15_messaging"
)

func TestPartitionedMessageRouter_PreservesPerKeyOrder(t *testing.T) {
	var mu sync.Mutex
	userSequences := make(map[string][]int64)

	handler := func(ctx context.Context, msg messaging.OrderedMessage) error {
		mu.Lock()
		userSequences[msg.PartitionKey] = append(userSequences[msg.PartitionKey], msg.Sequence)
		mu.Unlock()
		return nil
	}

	router := messaging.NewPartitionedMessageRouter(context.Background(), 4, handler)

	// Send sequences for multiple users interleaved
	for i := int64(1); i <= 10; i++ {
		router.Route(messaging.OrderedMessage{
			PartitionKey: "user-A",
			Sequence:     i,
			Payload:      fmt.Sprintf("User A Step %d", i),
		})
		router.Route(messaging.OrderedMessage{
			PartitionKey: "user-B",
			Sequence:     i,
			Payload:      fmt.Sprintf("User B Step %d", i),
		})
	}

	router.Stop()

	// Verify User A and User B both have strictly monotonic sequence 1..10
	for _, user := range []string{"user-A", "user-B"} {
		seqs := userSequences[user]
		if len(seqs) != 10 {
			t.Fatalf("expected 10 messages for %s, got: %d", user, len(seqs))
		}
		for i := range 10 {
			if seqs[i] != int64(i+1) {
				t.Errorf("%s out of order at index %d: expected %d, got %d", user, i, i+1, seqs[i])
			}
		}
	}
}
