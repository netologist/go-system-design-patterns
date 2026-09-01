package observability_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	observability "system-design-patterns/patterns/17_observability"
)

func TestStructuredLogger_Redaction(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := observability.NewStructuredLogger(buf, 1.0, observability.LevelInfo)

	fields := map[string]any{
		"user_id":     "usr-100",
		"db_password": "super-secret-password",
		"auth_token":  "bearer-token-12345",
	}

	logger.Log(context.Background(), observability.LevelInfo, "User login attempt", fields)

	var logRec observability.LogRecord
	if err := json.Unmarshal(buf.Bytes(), &logRec); err != nil {
		t.Fatalf("failed to parse log JSON: %v", err)
	}

	if logRec.Fields["db_password"] != "[REDACTED]" {
		t.Errorf("expected db_password redacted, got: %v", logRec.Fields["db_password"])
	}
	if logRec.Fields["auth_token"] != "[REDACTED]" {
		t.Errorf("expected auth_token redacted, got: %v", logRec.Fields["auth_token"])
	}
	if logRec.Fields["user_id"] != "usr-100" {
		t.Errorf("user_id should not be redacted")
	}
}

func TestStructuredLogger_LevelFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := observability.NewStructuredLogger(buf, 1.0, observability.LevelWarn)

	logger.Log(context.Background(), observability.LevelDebug, "Debug info", nil)
	logger.Log(context.Background(), observability.LevelInfo, "Info message", nil)

	if buf.Len() != 0 {
		t.Errorf("expected debug and info logs to be dropped, got: %s", buf.String())
	}

	logger.Log(context.Background(), observability.LevelWarn, "Warning message", nil)
	if !strings.Contains(buf.String(), "Warning message") {
		t.Errorf("expected warn log present, got: %s", buf.String())
	}
}
