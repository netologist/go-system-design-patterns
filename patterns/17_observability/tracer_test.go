package observability_test

import (
	"context"
	"testing"

	observability "system-design-patterns/patterns/17_observability"
)

func TestTracer_ParentChildSpanPropagation(t *testing.T) {
	ctx := context.Background()

	// 1. Root Span (HTTP Handler)
	rootSpan, rootCtx := observability.StartSpan(ctx, "HTTP GET /orders")
	rootSpan.SetTag("http.method", "GET")
	rootSpan.SetTag("http.status_code", "200")

	if rootSpan.TraceID == "" || rootSpan.SpanID == "" {
		t.Fatal("expected non-empty trace and span IDs")
	}
	if rootSpan.ParentSpanID != "" {
		t.Errorf("root span should have no parent, got: %s", rootSpan.ParentSpanID)
	}

	// 2. Child Span (DB Query)
	childSpan, _ := observability.StartSpan(rootCtx, "DB SELECT * FROM orders")
	childSpan.SetTag("db.system", "postgresql")
	childSpan.AddEvent("query_executed")
	childSpan.Finish()
	rootSpan.Finish()

	// Verify child shares identical TraceID with parent
	if childSpan.TraceID != rootSpan.TraceID {
		t.Errorf("child span must inherit root TraceID: child(%s) != root(%s)",
			childSpan.TraceID, rootSpan.TraceID)
	}

	// Verify child's ParentSpanID matches root's SpanID
	if childSpan.ParentSpanID != rootSpan.SpanID {
		t.Errorf("child ParentSpanID must match root SpanID: child.parent(%s) != root.span(%s)",
			childSpan.ParentSpanID, rootSpan.SpanID)
	}

	if childSpan.Tags["db.system"] != "postgresql" {
		t.Errorf("tag mismatch: %+v", childSpan.Tags)
	}
	if len(childSpan.Events) != 1 || childSpan.Events[0].Name != "query_executed" {
		t.Errorf("events mismatch: %+v", childSpan.Events)
	}
}
