package contextpattern

import (
	"context"
	"fmt"
	"time"
)

// RequestMetadata contains tracing & identity info propagated across boundaries.
type RequestMetadata struct {
	RequestID     string
	CorrelationID string
	CallerService string
}

// DownstreamDB simulates a database query checking context cancellation.
type DownstreamDB interface {
	Query(ctx context.Context, sql string) (string, error)
}

// DownstreamAPI simulates an external REST call checking context cancellation.
type DownstreamAPI interface {
	Call(ctx context.Context, endpoint string) (string, error)
}

// OrderWorkflow coordinates multi-stage distributed work with context propagation.
type OrderWorkflow struct {
	db  DownstreamDB
	api DownstreamAPI
}

func NewOrderWorkflow(db DownstreamDB, api DownstreamAPI) *OrderWorkflow {
	return &OrderWorkflow{db: db, api: api}
}

func (w *OrderWorkflow) Execute(ctx context.Context, orderID string) (string, error) {
	// 1. Context validation at entry
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("workflow aborted before start: %w", err)
	}

	// 2. Query database with propagated context
	dbResult, err := w.db.Query(ctx, fmt.Sprintf("SELECT * FROM orders WHERE id = '%s'", orderID))
	if err != nil {
		return "", fmt.Errorf("database stage failed: %w", err)
	}

	// 3. Call downstream service with propagated context
	apiResult, err := w.api.Call(ctx, "/v1/payments/verify")
	if err != nil {
		return "", fmt.Errorf("payment api stage failed: %w", err)
	}

	return fmt.Sprintf("Success: DB[%s] API[%s]", dbResult, apiResult), nil
}

// MockSlowDB simulates slow database query responding to context cancellation.
type MockSlowDB struct {
	Delay time.Duration
}

func (m *MockSlowDB) Query(ctx context.Context, sql string) (string, error) {
	select {
	case <-time.After(m.Delay):
		return "db_record_data", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// MockSlowAPI simulates slow external API responding to context cancellation.
type MockSlowAPI struct {
	Delay time.Duration
}

func (m *MockSlowAPI) Call(ctx context.Context, endpoint string) (string, error) {
	select {
	case <-time.After(m.Delay):
		return "api_verified", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
