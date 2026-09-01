package httpclient

import (
	"net"
	"net/http"
	"time"
)

// PooledClientConfig encapsulates connection pooling parameters.
type PooledClientConfig struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
	ConnectTimeout      time.Duration
	TLSHandshakeTimeout time.Duration
	OverallTimeout      time.Duration
}

// DefaultPooledClientConfig provides hardened production defaults.
func DefaultPooledClientConfig() PooledClientConfig {
	return PooledClientConfig{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		MaxConnsPerHost:     100, // Protect against downstream connection storms
		IdleConnTimeout:     90 * time.Second,
		ConnectTimeout:      5 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		OverallTimeout:      10 * time.Second,
	}
}

// NewPooledHTTPClient creates an *http.Client configured for high-concurrency reuse.
func NewPooledHTTPClient(cfg PooledClientConfig) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.OverallTimeout,
	}
}
