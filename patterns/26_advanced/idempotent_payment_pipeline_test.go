package advanced_test

import (
	"context"
	"sync/atomic"
	"testing"

	advanced "system-design-patterns/patterns/26_advanced"
)

type mockChargeGateway struct {
	chargeCount atomic.Int32
}

func (m *mockChargeGateway) Charge(ctx context.Context, customerID string, amount int64) (string, error) {
	m.chargeCount.Add(1)
	return "ch_stripe_999", nil
}

func TestIdempotentPaymentPipeline_IdempotencyReplay(t *testing.T) {
	gw := &mockChargeGateway{}
	pipeline := advanced.NewIdempotentPaymentPipeline(gw)
	ctx := context.Background()

	req := advanced.PaymentRequest{
		IdempotencyKey: "idemp_pay_abc123",
		Amount:         5000,
		CustomerID:     "cust_1",
	}

	// 1st request
	resp1, err := pipeline.ProcessPayment(ctx, req)
	if err != nil || resp1.Status != "SUCCEEDED" {
		t.Fatalf("first payment failed: %v", err)
	}

	if gw.chargeCount.Load() != 1 {
		t.Errorf("expected 1 charge on gateway, got: %d", gw.chargeCount.Load())
	}

	// 2nd request (client retry due to dropped network response)
	resp2, err := pipeline.ProcessPayment(ctx, req)
	if err != nil || resp2.PaymentID != resp1.PaymentID {
		t.Fatalf("replay payment failed: %v", err)
	}

	// Gateway charge count must still be 1 (No duplicate charge!)
	if gw.chargeCount.Load() != 1 {
		t.Fatalf("CRITICAL FINANCIAL BUG: duplicate charge executed! gateway count=%d", gw.chargeCount.Load())
	}
}
