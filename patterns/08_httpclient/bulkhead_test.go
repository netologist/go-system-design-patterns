package httpclient_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	httpclient "system-design-patterns/patterns/08_httpclient"
)

func TestBulkheadRegistry_WorkloadIsolation(t *testing.T) {
	registry := httpclient.NewBulkheadRegistry()
	registry.RegisterPartition("payments", 2)
	registry.RegisterPartition("emails", 5)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Block both payment slots
	startedCh := make(chan struct{})
	releasePaymentsCh := make(chan struct{})

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.Execute(ctx, "payments", func(c context.Context) error {
				startedCh <- struct{}{}
				<-releasePaymentsCh
				return nil
			})
		}()
	}

	// Wait for 2 payment workers to acquire slots
	<-startedCh
	<-startedCh

	// 3rd payment call must be rejected because payment capacity is 2
	err := registry.Execute(ctx, "payments", func(c context.Context) error {
		return nil
	})
	if !errors.Is(err, httpclient.ErrBulkheadFull) {
		t.Errorf("expected ErrBulkheadFull on saturated payments partition, got: %v", err)
	}

	// Meanwhile, emails partition MUST NOT be impacted and must succeed
	emailSuccess := false
	err = registry.Execute(ctx, "emails", func(c context.Context) error {
		emailSuccess = true
		return nil
	})
	if err != nil || !emailSuccess {
		t.Errorf("emails partition failed despite isolation: %v", err)
	}

	// Unblock payments
	close(releasePaymentsCh)
	wg.Wait()
}
