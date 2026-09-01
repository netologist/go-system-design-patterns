package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "system-design-patterns/patterns/11_middleware"
)

func TestAccessLoggerMiddleware(t *testing.T) {
	var loggedEntry *middleware.AccessLogEntry

	logger := func(entry middleware.AccessLogEntry) {
		loggedEntry = &entry
	}

	mw := middleware.AccessLoggerMiddleware(logger)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("Hello, World!"))
	}))

	req := httptest.NewRequest("POST", "/api/v1/users", nil)
	req.Header.Set("User-Agent", "TestRunner/1.0")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if loggedEntry == nil {
		t.Fatal("expected access log entry to be recorded")
	}

	if loggedEntry.Method != "POST" || loggedEntry.Path != "/api/v1/users" {
		t.Errorf("logged path/method mismatch: %+v", loggedEntry)
	}
	if loggedEntry.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got: %d", loggedEntry.StatusCode)
	}
	if loggedEntry.BytesWritten != 13 { // len("Hello, World!") = 13
		t.Errorf("expected 13 bytes written, got: %d", loggedEntry.BytesWritten)
	}
}
