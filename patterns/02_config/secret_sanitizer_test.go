package config_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	config "system-design-patterns/patterns/02_config"
)

type ConfigPayload struct {
	AppName  string                  `json:"app_name"`
	APIKey   config.Secret[string]   `json:"api_key"`
	Password config.Secret[string]   `json:"password"`
}

func TestSecret_Masking(t *testing.T) {
	secretPass := config.NewSecret("super-secret-password-123")

	// String format
	strOut := fmt.Sprintf("pass: %s", secretPass)
	if strings.Contains(strOut, "super-secret-password-123") || !strings.Contains(strOut, config.RedactedPlaceholder) {
		t.Errorf("expected redacted output, got: %s", strOut)
	}

	// GoString format (%#v)
	goStrOut := fmt.Sprintf("pass: %#v", secretPass)
	if strings.Contains(goStrOut, "super-secret-password-123") {
		t.Errorf("expected redacted GoString, got: %s", goStrOut)
	}

	// JSON Marshalling
	payload := ConfigPayload{
		AppName:  "PaymentService",
		APIKey:   config.NewSecret("sk_live_abc123"),
		Password: secretPass,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	jsonStr := string(bytes)
	if strings.Contains(jsonStr, "sk_live_abc123") || strings.Contains(jsonStr, "super-secret-password-123") {
		t.Errorf("secret leaked in json payload: %s", jsonStr)
	}

	// Expose returns original
	if secretPass.Expose() != "super-secret-password-123" {
		t.Errorf("Expose() did not return original secret value")
	}
}

func TestSanitizeMap(t *testing.T) {
	input := map[string]string{
		"PORT":          "8080",
		"DB_PASSWORD":   "my-secret-pw",
		"API_AUTH_KEY":  "bearer-token",
		"METRICS_ROUTE": "/metrics",
	}

	sanitized := config.SanitizeMap(input)
	if sanitized["PORT"] != "8080" {
		t.Errorf("expected PORT to remain unchanged")
	}
	if sanitized["DB_PASSWORD"] != config.RedactedPlaceholder {
		t.Errorf("expected DB_PASSWORD to be redacted")
	}
	if sanitized["API_AUTH_KEY"] != config.RedactedPlaceholder {
		t.Errorf("expected API_AUTH_KEY to be redacted")
	}
}

func TestRedactURL(t *testing.T) {
	raw := "postgres://admin:secret123@db.prod.internal:5432/mydb"
	redacted := config.RedactURL(raw)
	if strings.Contains(redacted, "secret123") {
		t.Errorf("expected credentials removed, got: %s", redacted)
	}
}
