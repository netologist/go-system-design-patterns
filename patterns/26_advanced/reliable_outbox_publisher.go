package advanced

import (
	"context"
	"errors"
	"sync"
	"time"

	messaging "system-design-patterns/patterns/15_messaging"
)

// BrokerMessageProducer simulates publishing to Kafka/RabbitMQ.
type BrokerMessageProducer interface {
	Publish(ctx context.Context, topic string, payload string) error
}

// ReliableOutboxPublisher continuously polls and flushes outbox events to the message broker.
type ReliableOutboxPublisher struct {
	store    *messaging.TransactionalOutboxStore
	producer BrokerMessageProducer
	interval time.Duration
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewReliableOutboxPublisher(
	parentCtx context.Context,
	store *messaging.TransactionalOutboxStore,
	producer BrokerMessageProducer,
	pollInterval time.Duration,
) *ReliableOutboxPublisher {
	if pollInterval <= 0 {
		pollInterval = 50 * time.Millisecond
	}

	ctx, cancel := context.WithCancel(parentCtx)

	return &ReliableOutboxPublisher{
		store:    store,
		producer: producer,
		interval: pollInterval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start launches the background relay publisher.
func (p *ReliableOutboxPublisher) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				_ = p.FlushOnce(p.ctx)
			}
		}
	}()
}

// FlushOnce processes one batch of pending outbox events.
func (p *ReliableOutboxPublisher) FlushOnce(ctx context.Context) error {
	pending := p.store.GetPendingEvents(ctx, 20)
	var errs []error

	for _, e := range pending {
		if err := p.producer.Publish(ctx, e.EventType, e.Payload); err != nil {
			errs = append(errs, err)
			continue // Will retry on next tick
		}
		_ = p.store.MarkPublished(ctx, e.ID)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Stop shuts down the background publisher.
func (p *ReliableOutboxPublisher) Stop() {
	p.cancel()
	p.wg.Wait()
}
