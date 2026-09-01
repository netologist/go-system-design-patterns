package config

import (
	"sync/atomic"
	"time"
)

// AppSettings defines the core settings.
type AppSettings struct {
	Environment string
	Port        int
	Features    map[string]bool
	Endpoints   []string
	Timeout     time.Duration
}

// ImmutableConfig provides safe, read-only access to application configuration.
type ImmutableConfig struct {
	env       string
	port      int
	features  map[string]bool
	endpoints []string
	timeout   time.Duration
}

// NewImmutableConfig creates an immutable snapshot by defensively copying collections.
func NewImmutableConfig(s AppSettings) *ImmutableConfig {
	featCopy := make(map[string]bool, len(s.Features))
	for k, v := range s.Features {
		featCopy[k] = v
	}

	epCopy := make([]string, len(s.Endpoints))
	copy(epCopy, s.Endpoints)

	return &ImmutableConfig{
		env:       s.Environment,
		port:      s.Port,
		features:  featCopy,
		endpoints: epCopy,
		timeout:   s.Timeout,
	}
}

func (c *ImmutableConfig) Environment() string { return c.env }
func (c *ImmutableConfig) Port() int          { return c.port }
func (c *ImmutableConfig) Timeout() time.Duration { return c.timeout }

// FeatureEnabled checks feature flag safely.
func (c *ImmutableConfig) FeatureEnabled(name string) bool {
	return c.features[name]
}

// Endpoints returns a defensive copy to prevent caller mutation.
func (c *ImmutableConfig) Endpoints() []string {
	res := make([]string, len(c.endpoints))
	copy(res, c.endpoints)
	return res
}

// ConfigHolder supports atomic replacement of immutable configs (Hot-Reload safe).
type ConfigHolder struct {
	current atomic.Pointer[ImmutableConfig]
}

// NewConfigHolder creates a holder initialized with the given immutable config.
func NewConfigHolder(initial *ImmutableConfig) *ConfigHolder {
	h := &ConfigHolder{}
	h.current.Store(initial)
	return h
}

// Get returns the current immutable config instance.
func (h *ConfigHolder) Get() *ImmutableConfig {
	return h.current.Load()
}

// Swap atomically replaces the active configuration.
func (h *ConfigHolder) Swap(newConfig *ImmutableConfig) {
	h.current.Store(newConfig)
}
