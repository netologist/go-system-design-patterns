package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpserver "system-design-patterns/patterns/07_httpserver"
)

func TestPanicRecoveryMiddleware_RecoversAndLogs(t *testing.T) {
	var loggedPanic any
	var loggedStack []byte

	logger := func(r *http.Request, recovered any, stack []byte) {
		loggedPanic = recovered
		loggedStack = stack
	}

	middleware := httpserver.PanicRecoveryMiddleware(logger)

	panickingHandler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("nil pointer dereference simulation")
	}))

	req := httptest.NewRequest(http.MethodGet, "/crash", nil)
	w := httptest.NewRecorder()

	panickingHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected HTTP 500 on panic, got: %d", w.Code)
	}

	if loggedPanic != "nil pointer dereference simulation" {
		t.Errorf("expected panic value logged, got: %v", loggedPanic)
	}

	if len(loggedStack) == 0 {
		t.Errorf("expected non-empty stack trace captured")
	}

	body := w.Body.String()
	if !strings.Contains(body, "internal_server_error") {
		t.Errorf("expected error JSON response, got: %s", body)
	}
}
