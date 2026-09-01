package messaging

import (
	"context"
	"hash/fnv"
	"sync"
)

// OrderedMessage represents a partition-keyed message.
type OrderedMessage struct {
	PartitionKey string
	Sequence     int64
	Payload      string
}

type partitionWorker struct {
	queue chan OrderedMessage
}

// PartitionedMessageRouter routes messages by PartitionKey to dedicated single-worker queues, guaranteeing per-key FIFO ordering.
type PartitionedMessageRouter struct {
	partitions []*partitionWorker
	numPartitions int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewPartitionedMessageRouter(
	parentCtx context.Context,
	numPartitions int,
	handler func(ctx context.Context, msg OrderedMessage) error,
) *PartitionedMessageRouter {
	if numPartitions <= 0 {
		numPartitions = 4
	}

	ctx, cancel := context.WithCancel(parentCtx)

	router := &PartitionedMessageRouter{
		partitions:    make([]*partitionWorker, numPartitions),
		numPartitions: numPartitions,
		ctx:           ctx,
		cancel:        cancel,
	}

	for i := range numPartitions {
		worker := &partitionWorker{
			queue: make(chan OrderedMessage, 100),
		}
		router.partitions[i] = worker

		router.wg.Add(1)
		go func(w *partitionWorker) {
			defer router.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-w.queue:
					if !ok {
						return
					}
					_ = handler(ctx, msg)
				}
			}
		}(worker)
	}

	return router
}

// Route hashes the partition key to select the dedicated partition queue.
func (r *PartitionedMessageRouter) Route(msg OrderedMessage) {
	idx := r.hashKey(msg.PartitionKey) % r.numPartitions
	r.partitions[idx].queue <- msg
}

func (r *PartitionedMessageRouter) hashKey(key string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32())
}

// Stop closes all partition channels and waits for workers to finish.
func (r *PartitionedMessageRouter) Stop() {
	for _, p := range r.partitions {
		close(p.queue)
	}
	r.wg.Wait()
	r.cancel()
}
