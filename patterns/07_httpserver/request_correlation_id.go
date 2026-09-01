package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type (
	requestIDCtxKey     struct{}
	correlationIDCtxKey struct{}
)

const (
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
)

// GenerateID produces a random 16-byte hex identifier.
func GenerateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestAndCorrelationIDMiddleware extracts or generates Request ID and Correlation ID.
func RequestAndCorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Request ID (unique to this HTTP request)
		reqID := strings.TrimSpace(r.Header.Get(HeaderRequestID))
		if reqID == "" {
			reqID = GenerateID()
		}

		// 2. Correlation ID (propagated across distributed services)
		corrID := strings.TrimSpace(r.Header.Get(HeaderCorrelationID))
		if corrID == "" {
			corrID = reqID // Default to request ID if first service in chain
		}

		// Inject into context
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, reqID)
		ctx = context.WithValue(ctx, correlationIDCtxKey{}, corrID)

		// Set response headers
		w.Header().Set(HeaderRequestID, reqID)
		w.Header().Set(HeaderCorrelationID, corrID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestIDFromContext extracts request ID from request context.
func GetRequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// GetCorrelationIDFromContext extracts correlation ID from request context.
func GetCorrelationIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(correlationIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}
