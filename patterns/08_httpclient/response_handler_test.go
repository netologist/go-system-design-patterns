package httpclient_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	httpclient "system-design-patterns/patterns/08_httpclient"
)

func TestSafeReadResponseBody_Success(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"status":"success"}`)),
	}

	data, err := httpclient.SafeReadResponseBody(resp, 1024)
	if err != nil {
		t.Fatalf("expected successful read, got: %v", err)
	}

	if string(data) != `{"status":"success"}` {
		t.Errorf("data mismatch: %s", string(data))
	}
}

func TestSafeReadResponseBody_ExceedsLimit(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("X", 100))),
	}

	_, err := httpclient.SafeReadResponseBody(resp, 50) // 50 bytes limit
	if !errors.Is(err, httpclient.ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge, got: %v", err)
	}
}

func TestValidateStatusCode(t *testing.T) {
	// 1. Valid 200
	resp200 := &http.Response{StatusCode: http.StatusOK}
	if err := httpclient.ValidateStatusCode(resp200); err != nil {
		t.Errorf("expected 200 to validate, got: %v", err)
	}

	// 2. 500 fails default
	resp500 := &http.Response{StatusCode: http.StatusInternalServerError}
	if err := httpclient.ValidateStatusCode(resp500); err == nil {
		t.Errorf("expected 500 to fail validation")
	}

	// 3. Allowed specific 404
	resp404 := &http.Response{StatusCode: http.StatusNotFound}
	if err := httpclient.ValidateStatusCode(resp404, http.StatusOK, http.StatusNotFound); err != nil {
		t.Errorf("expected 404 to be allowed in explicit list, got: %v", err)
	}
}
