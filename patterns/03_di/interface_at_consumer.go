package di

import (
	"context"
	"errors"
	"fmt"
)

// Order represents an e-commerce order.
type Order struct {
	ID        string
	Amount    int64 // In cents
	Status    string
	CustomerID string
}

// Consumer-defined interface: Only what OrderProcessor needs to read and write.
type OrderStorage interface {
	GetOrder(ctx context.Context, id string) (*Order, error)
	UpdateOrderStatus(ctx context.Context, id, status string) error
}

// Consumer-defined interface: Only what OrderProcessor needs to process payment.
type PaymentGateway interface {
	Charge(ctx context.Context, customerID string, amount int64) (string, error)
}

// OrderProcessor only depends on its narrow, consumer-defined interfaces.
type OrderProcessor struct {
	storage OrderStorage
	payment PaymentGateway
}

// NewOrderProcessor creates a processor with consumer-scoped interfaces.
func NewOrderProcessor(storage OrderStorage, payment PaymentGateway) *OrderProcessor {
	return &OrderProcessor{
		storage: storage,
		payment: payment,
	}
}

// ProcessOrder fetches order, executes payment, and transitions order status.
func (p *OrderProcessor) ProcessOrder(ctx context.Context, orderID string) error {
	order, err := p.storage.GetOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("fetch order error: %w", err)
	}

	if order.Status == "PAID" {
		return errors.New("order is already paid")
	}

	txnID, err := p.payment.Charge(ctx, order.CustomerID, order.Amount)
	if err != nil {
		_ = p.storage.UpdateOrderStatus(ctx, orderID, "PAYMENT_FAILED")
		return fmt.Errorf("payment declined: %w", err)
	}

	if txnID == "" {
		return errors.New("empty transaction id from payment provider")
	}

	return p.storage.UpdateOrderStatus(ctx, orderID, "PAID")
}
