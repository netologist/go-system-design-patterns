package database

import (
	"context"
	"errors"
	"fmt"
)

// PaymentOrder represents an order to be charged and recorded.
type PaymentOrder struct {
	ID     string
	Amount int64
	Status string
}

// PaymentTxScope provides atomic database operations.
type PaymentTxScope interface {
	SaveOrder(ctx context.Context, order *PaymentOrder) error
	DeductInventory(ctx context.Context, productID string, quantity int) error
}

// ExternalPaymentGateway performs network call outside of database transaction.
type ExternalPaymentGateway interface {
	ProcessPayment(ctx context.Context, orderID string, amount int64) (string, error)
}

// OrderCheckoutCoordinator enforces:
// 1. Pre-transaction validation
// 2. Network call to payment gateway OUTSIDE of DB transaction
// 3. Ultra-short DB transaction boundary for atomicity
// 4. Post-commit notification
type OrderCheckoutCoordinator struct {
	gateway ExternalPaymentGateway
}

func NewOrderCheckoutCoordinator(gw ExternalPaymentGateway) *OrderCheckoutCoordinator {
	return &OrderCheckoutCoordinator{gateway: gw}
}

func (c *OrderCheckoutCoordinator) Checkout(
	ctx context.Context,
	order *PaymentOrder,
	productID string,
	qty int,
	execTx func(ctx context.Context, fn func(tx PaymentTxScope) error) error,
) error {
	// Step 1: Pre-validation
	if order == nil || order.Amount <= 0 {
		return errors.New("invalid order amount")
	}

	// Step 2: NETWORK CALL OUTSIDE TRANSACTION
	// CRITICAL RULE: NEVER hold a DB transaction open across external HTTP/RPC network calls!
	paymentTxnID, err := c.gateway.ProcessPayment(ctx, order.ID, order.Amount)
	if err != nil {
		return fmt.Errorf("external payment declined: %w", err)
	}

	// Step 3: SHORT TRANSACTION BOUNDARY (DB operations only)
	err = execTx(ctx, func(tx PaymentTxScope) error {
		order.Status = "PAID"
		if err := tx.SaveOrder(ctx, order); err != nil {
			return err
		}
		return tx.DeductInventory(ctx, productID, qty)
	})

	if err != nil {
		// Log compensation necessity
		return fmt.Errorf("db transaction failed after payment txn %s: %w", paymentTxnID, err)
	}

	return nil
}
