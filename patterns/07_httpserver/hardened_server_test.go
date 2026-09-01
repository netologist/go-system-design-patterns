package httpserver_test

import (
	"net/http"
	"testing"
	"time"

	httpserver "system-design-patterns/patterns/07_httpserver"
)

func TestHardenedHTTPServer_Configuration(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	cfg := httpserver.DefaultHardenedServerConfig(":8080")

	srv := httpserver.NewHardenedHTTPServer(cfg, dummyHandler)

	if srv.Addr != ":8080" {
		t.Errorf("expected Addr ':8080', got: %s", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 2*time.Second {
		t.Errorf("expected ReadHeaderTimeout 2s, got: %v", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 5*time.Second {
		t.Errorf("expected ReadTimeout 5s, got: %v", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout 10s, got: %v", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 120*time.Second {
		t.Errorf("expected IdleTimeout 120s, got: %v", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 1<<20 {
		t.Errorf("expected MaxHeaderBytes 1MB, got: %d", srv.MaxHeaderBytes)
	}
}
