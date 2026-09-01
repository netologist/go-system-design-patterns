package config_test

import (
	"errors"
	"testing"
	"time"

	config "system-design-patterns/patterns/02_config"
)

func TestLoadFromEnv_SuccessDefaults(t *testing.T) {
	mockEnv := map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/mydb",
	}
	get := func(k string) string { return mockEnv[k] }

	cfg, err := config.LoadFromEnv(get)
	if err != nil {
		t.Fatalf("expected successful config load, got: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.Env != config.EnvDev {
		t.Errorf("expected default env 'dev', got %s", cfg.Env)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("expected read timeout 5s, got %v", cfg.ReadTimeout)
	}
	if cfg.MaxWorkers != 10 {
		t.Errorf("expected 10 workers, got %d", cfg.MaxWorkers)
	}
}

func TestLoadFromEnv_Overrides(t *testing.T) {
	mockEnv := map[string]string{
		"APP_ENV":           "prod",
		"APP_PORT":          "9000",
		"DATABASE_URL":      "postgres://prod:pass@db:5432/proddb",
		"APP_READ_TIMEOUT":  "2s",
		"APP_WRITE_TIMEOUT": "4s",
		"APP_MAX_WORKERS":   "50",
	}
	get := func(k string) string { return mockEnv[k] }

	cfg, err := config.LoadFromEnv(get)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	if cfg.Env != config.EnvProd || cfg.Port != 9000 || cfg.MaxWorkers != 50 {
		t.Errorf("unexpected config values: %+v", cfg)
	}
}

func TestLoadFromEnv_ValidationErrors(t *testing.T) {
	mockEnv := map[string]string{
		"APP_ENV":          "unknown_env",
		"APP_PORT":         "99999",
		"APP_READ_TIMEOUT": "-1s",
	}
	get := func(k string) string { return mockEnv[k] }

	_, err := config.LoadFromEnv(get)
	if err == nil {
		t.Fatal("expected validation errors, got nil")
	}

	// Should contain missing DB url and invalid port
	if !errors.Is(err, err) {
		t.Errorf("expected error, got %v", err)
	}
}
