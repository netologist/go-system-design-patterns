package config_test

import (
	"testing"
	"time"

	config "system-design-patterns/patterns/02_config"
)

func TestConfigValidator_AllValid(t *testing.T) {
	v := config.NewConfigValidator()
	v.RequireNotEmpty("service_name", "orders-api")
	v.ValidatePort("http_port", 8080)
	v.ValidatePositiveDuration("timeout", 2*time.Second)
	v.ValidateURL("db_url", "postgres://user:pass@localhost:5432/db", "postgres", "postgresql")
	v.ValidateEnum("env", "staging", "dev", "staging", "prod")

	if err := v.Result(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

func TestConfigValidator_MultipleViolations(t *testing.T) {
	v := config.NewConfigValidator()
	v.RequireNotEmpty("service_name", "")
	v.ValidatePort("http_port", 70000)
	v.ValidatePositiveDuration("timeout", -5*time.Second)
	v.ValidateURL("db_url", "ftp://localhost/file", "postgres")
	v.ValidateEnum("env", "invalid_env", "dev", "prod")

	err := v.Result()
	if err == nil {
		t.Fatal("expected aggregated validation error, got nil")
	}
}
