package apidesign_test

import (
	"net/http/httptest"
	"testing"

	apidesign "system-design-patterns/patterns/10_apidesign"
)

func TestExtractAPIVersion(t *testing.T) {
	// 1. Path-based
	reqPath := httptest.NewRequest("GET", "/api/v2/products", nil)
	if v := apidesign.ExtractAPIVersion(reqPath); v != apidesign.VersionV2 {
		t.Errorf("expected v2 from path, got: %s", v)
	}

	// 2. Header-based
	reqHeader := httptest.NewRequest("GET", "/products", nil)
	reqHeader.Header.Set("X-API-Version", "2")
	if v := apidesign.ExtractAPIVersion(reqHeader); v != apidesign.VersionV2 {
		t.Errorf("expected v2 from header, got: %s", v)
	}

	// 3. Accept content-negotiation
	reqAccept := httptest.NewRequest("GET", "/products", nil)
	reqAccept.Header.Set("Accept", "application/vnd.acme.v2+json")
	if v := apidesign.ExtractAPIVersion(reqAccept); v != apidesign.VersionV2 {
		t.Errorf("expected v2 from Accept header, got: %s", v)
	}

	// 4. Default fallback
	reqDefault := httptest.NewRequest("GET", "/products", nil)
	if v := apidesign.ExtractAPIVersion(reqDefault); v != apidesign.VersionV1 {
		t.Errorf("expected default v1, got: %s", v)
	}
}
