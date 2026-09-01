package apidesign_test

import (
	"net/http/httptest"
	"testing"

	apidesign "system-design-patterns/patterns/10_apidesign"
)

func TestETag_IfNoneMatch(t *testing.T) {
	etag := apidesign.GenerateETag([]byte(`{"id": 1, "version": 2}`))

	req := httptest.NewRequest("GET", "/resource", nil)
	req.Header.Set("If-None-Match", etag)

	if !apidesign.CheckIfNoneMatch(req, etag) {
		t.Error("expected If-None-Match to match identical ETag")
	}

	reqMismatch := httptest.NewRequest("GET", "/resource", nil)
	reqMismatch.Header.Set("If-None-Match", `"different-tag"`)

	if apidesign.CheckIfNoneMatch(reqMismatch, etag) {
		t.Error("expected If-None-Match to report mismatch")
	}
}

func TestETag_IfMatch(t *testing.T) {
	etag := apidesign.GenerateETag([]byte(`{"id": 1, "version": 2}`))

	// Matching tag -> Valid
	req := httptest.NewRequest("PUT", "/resource", nil)
	req.Header.Set("If-Match", etag)
	if !apidesign.CheckIfMatch(req, etag) {
		t.Error("expected If-Match to match")
	}

	// Stale tag -> Precondition Failed
	reqStale := httptest.NewRequest("PUT", "/resource", nil)
	reqStale.Header.Set("If-Match", `"stale-etag-v1"`)
	if apidesign.CheckIfMatch(reqStale, etag) {
		t.Error("expected stale If-Match to be rejected")
	}
}
