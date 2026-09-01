package di_test

import (
	"context"
	"errors"
	"testing"

	di "system-design-patterns/patterns/03_di"
)

type mockOrderStorage struct {
	order      *di.Order
	lastStatus string
}

func (m *mockOrderStorage) GetOrder(ctx context.Context, id string) (*di.Order, error) {
	if m.order == nil {
		return nil, errors.New("order not found")
	}
	return m.order, nil
}

func (m *mockOrderStorage) UpdateOrderStatus(ctx context.Context, id, status string) error {
	m.lastStatus = status
	if m.order != nil {
		m.order.Status = status
	}
	return nil
}

type mockPaymentGateway struct {
	shouldFail bool
	txnID      string
}

func (m *mockPaymentGateway) Charge(ctx context.Context, customerID string, amount int64) (string, error) {
	if m.shouldFail {
		return "", errors.New("insufficient funds")
	}
	return m.txnID, nil
}

func TestOrderProcessor_Success(t *testing.T) {
	storage := &mockOrderStorage{
		order: &di.Order{
			ID:         "ord-123",
			Amount:     5000,
			Status:     "PENDING",
			CustomerID: "cust-99",
		},
	}
	payment := &mockPaymentGateway{txnID: "txn-abc"}

	proc := di.NewOrderProcessor(storage, payment)
	err := proc.ProcessOrder(context.Background(), "ord-123")
	if err != nil {
		t.Fatalf("expected order to process successfully, got: %v", err)
	}

	if storage.lastStatus != "PAID" {
		t.Errorf("expected order status PAID, got: %s", storage.lastStatus)
	}
}

func TestOrderProcessor_PaymentFailed(t *testing.T) {
	storage := &mockOrderStorage{
		order: &di.Order{
			ID:         "ord-456",
			Amount:     1200,
			Status:     "PENDING",
			CustomerID: "cust-99",
		},
	}
	payment := &mockPaymentGateway{shouldFail: true}

	proc := di.NewOrderProcessor(storage, payment)
	err := proc.ProcessOrder(context.Background(), "ord-456")
	if err == nil {
		t.Fatal("expected payment failure error, got nil")
	}

	if storage.lastStatus != "PAYMENT_FAILED" {
		t.Errorf("expected order status PAYMENT_FAILED, got: %s", storage.lastStatus)
	}
}
