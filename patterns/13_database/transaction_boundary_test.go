package database_test

import (
	"context"
	"errors"
	"testing"

	database "system-design-patterns/patterns/13_database"
)

type mockGateway struct {
	called bool
	err    error
}

func (m *mockGateway) ProcessPayment(ctx context.Context, orderID string, amount int64) (string, error) {
	m.called = true
	return "txn_stripe_123", m.err
}

type mockTxScope struct {
	savedOrder *database.PaymentOrder
	deducted   bool
}

func (m *mockTxScope) SaveOrder(ctx context.Context, order *database.PaymentOrder) error {
	m.savedOrder = order
	return nil
}

func (m *mockTxScope) DeductInventory(ctx context.Context, productID string, quantity int) error {
	m.deducted = true
	return nil
}

func TestOrderCheckoutCoordinator_OrderOfOperations(t *testing.T) {
	gw := &mockGateway{}
	coordinator := database.NewOrderCheckoutCoordinator(gw)

	txScope := &mockTxScope{}
	txStarted := false

	execTx := func(ctx context.Context, fn func(tx database.PaymentTxScope) error) error {
		txStarted = true
		return fn(txScope)
	}

	order := &database.PaymentOrder{ID: "ord-1", Amount: 5000}
	err := coordinator.Checkout(context.Background(), order, "prod-99", 1, execTx)

	if err != nil {
		t.Fatalf("checkout failed: %v", err)
	}

	if !gw.called {
		t.Error("expected payment gateway called")
	}
	if !txStarted || !txScope.deducted || txScope.savedOrder.Status != "PAID" {
		t.Errorf("expected transaction to complete successfully: %+v", txScope)
	}
}

func TestOrderCheckoutCoordinator_PaymentDeclinedPreventsTx(t *testing.T) {
	gw := &mockGateway{err: errors.New("insufficient funds")}
	coordinator := database.NewOrderCheckoutCoordinator(gw)

	txStarted := false
	execTx := func(ctx context.Context, fn func(tx database.PaymentTxScope) error) error {
		txStarted = true
		return nil
	}

	order := &database.PaymentOrder{ID: "ord-2", Amount: 1000}
	err := coordinator.Checkout(context.Background(), order, "prod-1", 1, execTx)

	if err == nil {
		t.Fatal("expected error on payment decline, got nil")
	}

	if txStarted {
		t.Error("DB transaction should NOT have been opened after payment decline")
	}
}
