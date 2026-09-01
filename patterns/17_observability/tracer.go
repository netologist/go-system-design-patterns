package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type spanCtxKey struct{}

// SpanEvent represents a timestamped milestone event within a span.
type SpanEvent struct {
	Name      string
	Timestamp time.Time
}

// Span represents a single unit of distributed work.
type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	StartTime    time.Time
	EndTime      time.Time
	Tags         map[string]string
	Events       []SpanEvent
	mu           sync.Mutex
}

func (s *Span) SetTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tags[key] = value
}

func (s *Span) AddEvent(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Events = append(s.Events, SpanEvent{
		Name:      name,
		Timestamp: time.Now(),
	})
}

func (s *Span) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EndTime = time.Now()
}

func generateHexID(bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// StartSpan starts a new span, linking to parent span in context if present.
func StartSpan(ctx context.Context, name string) (*Span, context.Context) {
	var parentSpanID string
	traceID := generateHexID(16) // 128-bit Trace ID

	if parent := SpanFromContext(ctx); parent != nil {
		traceID = parent.TraceID
		parentSpanID = parent.SpanID
	}

	spanID := generateHexID(8) // 64-bit Span ID
	span := &Span{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Name:         name,
		StartTime:    time.Now(),
		Tags:         make(map[string]string),
	}

	childCtx := context.WithValue(ctx, spanCtxKey{}, span)
	return span, childCtx
}

// SpanFromContext retrieves the active span from context.
func SpanFromContext(ctx context.Context) *Span {
	if s, ok := ctx.Value(spanCtxKey{}).(*Span); ok {
		return s
	}
	return nil
}
