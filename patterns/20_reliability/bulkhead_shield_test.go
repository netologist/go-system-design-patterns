package reliability_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	reliability "system-design-patterns/patterns/20_reliability"
)

func TestBulkheadShield_Isolation(t *testing.T) {
	shield := reliability.NewBulkheadShield()
	shield.RegisterDependency("stripe", 1, 100*time.Millisecond)
	shield.RegisterDependency("sendgrid", 5, 100*time.Millisecond)

	ctx := context.Background()

	// Occupy the only stripe slot
	blockStripe := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = shield.Call(ctx, "stripe", func(c context.Context) error {
			<-blockStripe
			return nil
		})
	}()

	time.Sleep(10 * time.Millisecond)

	// 2nd stripe call fails due to saturation
	err := shield.Call(ctx, "stripe", func(c context.Context) error {
		return nil
	})
	if !errors.Is(err, reliability.ErrDependencySaturated) {
		t.Errorf("expected ErrDependencySaturated on stripe, got: %v", err)
	}

	// Meanwhile sendgrid call succeeds
	sendgridSuccess := false
	err = shield.Call(ctx, "sendgrid", func(c context.Context) error {
		sendgridSuccess = true
		return nil
	})
	if err != nil || !sendgridSuccess {
		t.Errorf("sendgrid call failed despite isolation: %v", err)
	}

	close(blockStripe)
	wg.Wait()
}
