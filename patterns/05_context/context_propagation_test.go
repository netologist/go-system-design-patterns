package contextpattern_test

import (
	"context"
	"errors"
	"testing"
	"time"

	contextpattern "system-design-patterns/patterns/05_context"
)

func TestContextPropagation_Success(t *testing.T) {
	db := &contextpattern.MockSlowDB{Delay: 10 * time.Millisecond}
	api := &contextpattern.MockSlowAPI{Delay: 10 * time.Millisecond}
	wf := contextpattern.NewOrderWorkflow(db, api)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := wf.Execute(ctx, "ord-100")
	if err != nil {
		t.Fatalf("expected workflow to succeed, got: %v", err)
	}

	if result == "" {
		t.Errorf("expected non-empty result string")
	}
}

func TestContextPropagation_DBCancellation(t *testing.T) {
	// DB takes 200ms but context has only 30ms
	db := &contextpattern.MockSlowDB{Delay: 200 * time.Millisecond}
	api := &contextpattern.MockSlowAPI{Delay: 10 * time.Millisecond}
	wf := contextpattern.NewOrderWorkflow(db, api)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := wf.Execute(ctx, "ord-100")
	if err == nil {
		t.Fatal("expected deadline exceeded error during DB stage, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestContextPropagation_APICancellation(t *testing.T) {
	// DB is fast (10ms) but API takes 200ms; overall timeout 50ms
	db := &contextpattern.MockSlowDB{Delay: 10 * time.Millisecond}
	api := &contextpattern.MockSlowAPI{Delay: 200 * time.Millisecond}
	wf := contextpattern.NewOrderWorkflow(db, api)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := wf.Execute(ctx, "ord-100")
	if err == nil {
		t.Fatal("expected deadline exceeded error during API stage, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}
