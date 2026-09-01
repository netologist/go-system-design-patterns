package httpserver

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

var (
	ErrBodyTooLarge        = errors.New("request body exceeds maximum allowed size")
	ErrInvalidContentType  = errors.New("invalid or unsupported Content-Type header")
	ErrMissingContentType  = errors.New("missing Content-Type header")
)

// BodyLimitMiddleware enforces maximum request body size using http.MaxBytesReader.
func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireContentType verifies that the incoming request matches expected media type (e.g. application/json).
func RequireContentType(expectedMediaType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip check on body-less requests (GET, HEAD, OPTIONS, DELETE without body)
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			contentType := r.Header.Get("Content-Type")
			if strings.TrimSpace(contentType) == "" {
				http.Error(w, ErrMissingContentType.Error(), http.StatusUnsupportedMediaType)
				return
			}

			mediaType, _, err := mime.ParseMediaType(contentType)
			if err != nil || !strings.EqualFold(mediaType, expectedMediaType) {
				http.Error(w, fmt.Sprintf("%s (expected: %s, got: %s)", ErrInvalidContentType.Error(), expectedMediaType, contentType), http.StatusUnsupportedMediaType)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
