package advanced

import (
	"context"
	"errors"
	"fmt"
	"sync"

	dataintegrity "system-design-patterns/patterns/23_dataintegrity"
)

// PaymentRequest contains input for creating a payment.
type PaymentRequest struct {
	IdempotencyKey string
	Amount         int64
	CustomerID     string
}

// PaymentResponse represents finalized outcome.
type PaymentResponse struct {
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
}

type ExternalChargeGateway interface {
	Charge(ctx context.Context, customerID string, amount int64) (string, error)
}

// IdempotentPaymentPipeline coordinates complete safe payment execution.
type IdempotentPaymentPipeline struct {
	mu           sync.Mutex
	gateway      ExternalChargeGateway
	idempotency  map[string]*PaymentResponse
	inFlightKeys map[string]struct{}
}

func NewIdempotentPaymentPipeline(gw ExternalChargeGateway) *IdempotentPaymentPipeline {
	return &IdempotentPaymentPipeline{
		gateway:      gw,
		idempotency:  make(map[string]*PaymentResponse),
		inFlightKeys: make(map[string]struct{}),
	}
}

// ProcessPayment executes the complete resilient payment flow.
func (p *IdempotentPaymentPipeline) ProcessPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	if req.IdempotencyKey == "" {
		return nil, errors.New("idempotency key is required")
	}

	p.mu.Lock()
	// 1. Check existing idempotent result
	if cached, ok := p.idempotency[req.IdempotencyKey]; ok {
		p.mu.Unlock()
		return cached, nil
	}

	// 2. Lock key to prevent concurrent duplicate execution
	if _, inFlight := p.inFlightKeys[req.IdempotencyKey]; inFlight {
		p.mu.Unlock()
		return nil, errors.New("concurrent payment with same idempotency key in flight")
	}
	p.inFlightKeys[req.IdempotencyKey] = struct{}{}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.inFlightKeys, req.IdempotencyKey)
		p.mu.Unlock()
	}()

	// 3. State Machine check
	sm := dataintegrity.NewPaymentStateMachine(dataintegrity.StatePending)
	_ = sm.TransitionTo(dataintegrity.StateProcessing)

	// 4. NETWORK CALL TO PAYMENT GATEWAY (OUTSIDE DB TRANSACTION)
	chargeID, err := p.gateway.Charge(ctx, req.CustomerID, req.Amount)
	if err != nil {
		_ = sm.TransitionTo(dataintegrity.StateFailed)
		return nil, fmt.Errorf("charge failed: %w", err)
	}

	_ = sm.TransitionTo(dataintegrity.StateSucceeded)

	// 5. Build and cache response
	resp := &PaymentResponse{
		PaymentID: chargeID,
		Status:    string(sm.CurrentState()),
		Amount:    req.Amount,
	}

	p.mu.Lock()
	p.idempotency[req.IdempotencyKey] = resp
	p.mu.Unlock()

	return resp, nil
}
