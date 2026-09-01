package middleware

import (
	"net/http"
	"time"
)

// AccessLogEntry holds structured metadata about a completed HTTP request.
type AccessLogEntry struct {
	Method       string
	Path         string
	StatusCode   int
	BytesWritten int64
	Duration     time.Duration
	RemoteAddr   string
	UserAgent    string
}

// LogWriter is a callback invoked when an access log entry is generated.
type LogWriter func(entry AccessLogEntry)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

func (w *responseWriterWrapper) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// AccessLoggerMiddleware produces structured log records for each request.
func AccessLoggerMiddleware(writer LogWriter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default if WriteHeader is never called
			}

			next.ServeHTTP(wrapped, r)

			if writer != nil {
				writer(AccessLogEntry{
					Method:       r.Method,
					Path:         r.URL.Path,
					StatusCode:   wrapped.statusCode,
					BytesWritten: wrapped.bytesWritten,
					Duration:     time.Since(start),
					RemoteAddr:   r.RemoteAddr,
					UserAgent:    r.UserAgent(),
				})
			}
		})
	}
}
