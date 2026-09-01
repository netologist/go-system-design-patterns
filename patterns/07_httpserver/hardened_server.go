package httpserver

import (
	"net/http"
	"time"
)

// HardenedServerConfig defines production-grade timeout parameters to defend against Slowloris and resource retention.
type HardenedServerConfig struct {
	Addr              string
	ReadHeaderTimeout time.Duration // Crucial for Slowloris mitigation
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
}

// DefaultHardenedServerConfig provides production-ready defensive defaults.
func DefaultHardenedServerConfig(addr string) HardenedServerConfig {
	return HardenedServerConfig{
		Addr:              addr,
		ReadHeaderTimeout: 2 * time.Second,  // Fast rejection of slow-reading headers
		ReadTimeout:       5 * time.Second,  // Maximum time reading full request body
		WriteTimeout:      10 * time.Second, // Maximum time writing response
		IdleTimeout:       120 * time.Second,// Keep-alive idle connection lifetime
		MaxHeaderBytes:    1 << 20,          // 1 MB max header size
	}
}

// NewHardenedHTTPServer creates a standard *http.Server configured with hardened timeouts.
func NewHardenedHTTPServer(cfg HardenedServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
}
