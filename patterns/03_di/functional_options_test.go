package di_test

import (
	"testing"
	"time"

	di "system-design-patterns/patterns/03_di"
)

func TestNewClient_Defaults(t *testing.T) {
	client, err := di.NewClient("https://api.example.com")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.Timeout() != 5*time.Second {
		t.Errorf("expected default timeout 5s, got %v", client.Timeout())
	}
	if client.MaxRetries() != 3 {
		t.Errorf("expected default retries 3, got %d", client.MaxRetries())
	}
	if client.MaxIdleConns() != 100 {
		t.Errorf("expected default max idle conns 100, got %d", client.MaxIdleConns())
	}
}

func TestNewClient_CustomOptions(t *testing.T) {
	client, err := di.NewClient("https://api.example.com",
		di.WithTimeout(2*time.Second),
		di.WithMaxRetries(5),
		di.WithUserAgent("CustomAgent/2.0"),
		di.WithMaxIdleConns(50),
	)
	if err != nil {
		t.Fatalf("failed to create custom client: %v", err)
	}

	if client.Timeout() != 2*time.Second {
		t.Errorf("expected timeout 2s, got %v", client.Timeout())
	}
	if client.MaxRetries() != 5 {
		t.Errorf("expected retries 5, got %d", client.MaxRetries())
	}
	if client.UserAgent() != "CustomAgent/2.0" {
		t.Errorf("expected CustomAgent/2.0, got %s", client.UserAgent())
	}
	if client.MaxIdleConns() != 50 {
		t.Errorf("expected max idle conns 50, got %d", client.MaxIdleConns())
	}
}

func TestNewClient_InvalidOptions(t *testing.T) {
	_, err := di.NewClient("", di.WithTimeout(1*time.Second))
	if err == nil {
		t.Error("expected error for empty base URL")
	}

	_, err = di.NewClient("https://api.example.com", di.WithTimeout(-1*time.Second))
	if err == nil {
		t.Error("expected error for negative timeout")
	}

	_, err = di.NewClient("https://api.example.com", di.WithMaxRetries(-1))
	if err == nil {
		t.Error("expected error for negative retries")
	}
}
