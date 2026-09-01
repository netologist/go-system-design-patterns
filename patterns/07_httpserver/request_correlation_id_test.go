package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "system-design-patterns/patterns/07_httpserver"
)

func TestRequestAndCorrelationIDMiddleware_Generated(t *testing.T) {
	var ctxReqID, ctxCorrID string

	handler := httpserver.RequestAndCorrelationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxReqID = httpserver.GetRequestIDFromContext(r.Context())
		ctxCorrID = httpserver.GetCorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	respReqID := w.Header().Get(httpserver.HeaderRequestID)
	respCorrID := w.Header().Get(httpserver.HeaderCorrelationID)

	if respReqID == "" || respCorrID == "" {
		t.Fatal("expected request and correlation IDs in response headers")
	}

	if ctxReqID != respReqID || ctxCorrID != respCorrID {
		t.Errorf("context and header IDs do not match: ctx(%s, %s) resp(%s, %s)",
			ctxReqID, ctxCorrID, respReqID, respCorrID)
	}
}

func TestRequestAndCorrelationIDMiddleware_Propagated(t *testing.T) {
	var ctxCorrID string

	handler := httpserver.RequestAndCorrelationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxCorrID = httpserver.GetCorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(httpserver.HeaderCorrelationID, "existing-trace-xyz")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	respCorrID := w.Header().Get(httpserver.HeaderCorrelationID)
	if respCorrID != "existing-trace-xyz" || ctxCorrID != "existing-trace-xyz" {
		t.Errorf("expected propagated correlation ID 'existing-trace-xyz', got resp=%s ctx=%s",
			respCorrID, ctxCorrID)
	}
}
