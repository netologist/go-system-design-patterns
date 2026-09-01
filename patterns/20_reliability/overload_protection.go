package reliability

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

// OverloadProtectionMiddleware limits maximum concurrent active requests across the whole server.
// When capacity is exceeded, it returns controlled 503 Service Unavailable rather than collapsing the process.
func OverloadProtectionMiddleware(maxActiveRequests int64) func(http.Handler) http.Handler {
	if maxActiveRequests <= 0 {
		maxActiveRequests = 1000
	}

	var activeRequests atomic.Int64

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			current := activeRequests.Add(1)
			defer activeRequests.Add(-1)

			if current > maxActiveRequests {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "service_unavailable",
					"message": "Server capacity temporarily saturated. Please retry shortly.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
