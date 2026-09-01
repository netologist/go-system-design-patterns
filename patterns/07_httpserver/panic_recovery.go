package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
)

// PanicLogger is a callback invoked when a panic occurs.
type PanicLogger func(r *http.Request, recovered any, stack []byte)

// PanicRecoveryMiddleware catches any panic in downstream handlers and returns 500.
func PanicRecoveryMiddleware(logger PanicLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := debug.Stack()
					if logger != nil {
						logger(r, rec, stack)
					}

					// Return standard 500 internal server error
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error":   "internal_server_error",
						"message": fmt.Sprintf("An unexpected panic occurred: %v", rec),
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
