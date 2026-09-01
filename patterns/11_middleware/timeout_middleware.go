package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// TimeoutMiddleware applies a strict deadline to incoming request handlers.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			done := make(chan struct{})
			panicChan := make(chan any, 1)

			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			go func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicChan <- rec
					}
					close(done)
				}()
				next.ServeHTTP(wrapped, r.WithContext(ctx))
			}()

			select {
			case rec := <-panicChan:
				panic(rec) // Re-panic to allow recovery middleware to handle
			case <-done:
				// Successfully completed in time
				return
			case <-ctx.Done():
				// Deadline exceeded
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusGatewayTimeout)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "gateway_timeout",
					"message": "The request processing exceeded the configured timeout deadline",
				})
			}
		})
	}
}
