package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeadersMiddleware sets standard defensive browser security headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		h.Set("Content-Security-Policy", "default-src 'self'")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

// CORSConfig specifies allowed cross-origin settings.
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAgeSeconds  int
}

// CORSMiddleware handles cross-origin requests and OPTIONS preflights.
func CORSMiddleware(cfg CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				allowed := false
				for _, o := range cfg.AllowedOrigins {
					if o == "*" || strings.EqualFold(o, origin) {
						allowed = true
						w.Header().Set("Access-Control-Allow-Origin", origin)
						break
					}
				}

				if allowed {
					if len(cfg.AllowedMethods) > 0 {
						w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
					}
					if len(cfg.AllowedHeaders) > 0 {
						w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
					}
				}
			}

			// Intercept preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
