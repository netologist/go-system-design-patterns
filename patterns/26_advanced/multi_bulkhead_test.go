package advanced_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	advanced "system-design-patterns/patterns/26_advanced"
)

func TestMultiWorkloadBulkhead_Isolation(t *testing.T) {
	bulkhead := advanced.NewMultiWorkloadBulkhead(context.Background())
	defer bulkhead.Stop()

	var paymentCompleted atomic.Int32
	var emailCompleted atomic.Int32
	var kycCompleted atomic.Int32

	// Submit jobs to all three pools
	for range 5 {
		_ = bulkhead.Dispatch(advanced.WorkloadPayment, func(c context.Context) {
			paymentCompleted.Add(1)
		})
		_ = bulkhead.Dispatch(advanced.WorkloadEmail, func(c context.Context) {
			emailCompleted.Add(1)
		})
		_ = bulkhead.Dispatch(advanced.WorkloadKYC, func(c context.Context) {
			kycCompleted.Add(1)
		})
	}

	time.Sleep(30 * time.Millisecond)

	if paymentCompleted.Load() != 5 || emailCompleted.Load() != 5 || kycCompleted.Load() != 5 {
		t.Errorf("expected 5 completed in each pool, got pay=%d email=%d kyc=%d",
			paymentCompleted.Load(), emailCompleted.Load(), kycCompleted.Load())
	}
}
