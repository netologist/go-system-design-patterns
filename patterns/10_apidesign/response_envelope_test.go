package apidesign_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apidesign "system-design-patterns/patterns/10_apidesign"
)

func TestFilterUserOutput_StripsSensitiveFields(t *testing.T) {
	internalUser := &apidesign.UserAccount{
		ID:           "usr-secret-123",
		Email:        "  John.Doe@Example.COM  ",
		PasswordHash: "$2a$12$e8n...super_secret_hash",
		SSN:          "123-45-6789",
		CreatedAt:    time.Now(),
	}

	normalizedEmail := apidesign.NormalizeEmail(internalUser.Email)
	if normalizedEmail != "john.doe@example.com" {
		t.Errorf("expected normalized email 'john.doe@example.com', got: %s", normalizedEmail)
	}
	internalUser.Email = normalizedEmail

	dto := apidesign.FilterUserOutput(internalUser)

	bytes, _ := json.Marshal(dto)
	jsonStr := string(bytes)

	if strings.Contains(jsonStr, "super_secret_hash") || strings.Contains(jsonStr, "123-45-6789") {
		t.Fatalf("CRITICAL: sensitive fields leaked in public DTO output: %s", jsonStr)
	}
}

func TestWriteSuccessResponse_Envelope(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"greeting": "hello"}
	meta := &apidesign.APIMeta{Page: 1, Limit: 10, TotalItems: 1}

	apidesign.WriteSuccessResponse(w, http.StatusOK, data, meta)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got: %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"greeting":"hello"`) || !strings.Contains(body, `"total_items":1`) {
		t.Errorf("envelope missing data or meta: %s", body)
	}
}
