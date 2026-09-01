package advanced

import (
	"context"
	"errors"
	"sync"
)

var ErrWorkloadQueueFull = errors.New("workload bulkhead queue is full")

type WorkloadType string

const (
	WorkloadPayment WorkloadType = "PAYMENT"
	WorkloadEmail   WorkloadType = "EMAIL"
	WorkloadKYC     WorkloadType = "KYC"
)

// MultiWorkloadBulkhead manages separate isolated queues and worker pools for distinct application domains.
type MultiWorkloadBulkhead struct {
	queues map[WorkloadType]chan func(ctx context.Context)
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMultiWorkloadBulkhead(parentCtx context.Context) *MultiWorkloadBulkhead {
	ctx, cancel := context.WithCancel(parentCtx)
	b := &MultiWorkloadBulkhead{
		queues: map[WorkloadType]chan func(ctx context.Context){
			WorkloadPayment: make(chan func(ctx context.Context), 50),
			WorkloadEmail:   make(chan func(ctx context.Context), 20),
			WorkloadKYC:     make(chan func(ctx context.Context), 20),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Launch dedicated worker allocations: 10 Payment, 5 Email, 5 KYC
	b.spawnWorkers(WorkloadPayment, 10)
	b.spawnWorkers(WorkloadEmail, 5)
	b.spawnWorkers(WorkloadKYC, 5)

	return b
}

func (b *MultiWorkloadBulkhead) spawnWorkers(wType WorkloadType, workerCount int) {
	q := b.queues[wType]
	for range workerCount {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			for {
				select {
				case <-b.ctx.Done():
					return
				case job, ok := <-q:
					if !ok {
						return
					}
					job(b.ctx)
				}
			}
		}()
	}
}

// Dispatch submits a job to the workload's isolated queue.
func (b *MultiWorkloadBulkhead) Dispatch(wType WorkloadType, job func(ctx context.Context)) error {
	q, ok := b.queues[wType]
	if !ok {
		return errors.New("unknown workload type")
	}

	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case q <- job:
		return nil
	default:
		return ErrWorkloadQueueFull
	}
}

func (b *MultiWorkloadBulkhead) Stop() {
	b.cancel()
	b.wg.Wait()
}
