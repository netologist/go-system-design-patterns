package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AppConfig represents bootstrap configuration.
type AppConfig struct {
	Port            int
	ShutdownTimeout time.Duration
}

// AppDependencies represents instantiated infrastructure and domain components.
type AppDependencies struct {
	HTTPServer *http.Server
	Shutdown   *GracefulShutdownManager
}

// CompositionRoot builds and runs the application lifecycle.
type CompositionRoot struct {
	config AppConfig
	deps   *AppDependencies
	mu     sync.Mutex
}

// NewCompositionRoot initializes the composition root with given configuration.
func NewCompositionRoot(cfg AppConfig) *CompositionRoot {
	return &CompositionRoot{
		config: cfg,
	}
}

// Build wires the dependency graph.
func (c *CompositionRoot) Build(handler http.Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config.Port <= 0 {
		return errors.New("invalid port in configuration")
	}

	timeout := c.config.ShutdownTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	shutdownMgr := NewGracefulShutdownManager(timeout)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", c.config.Port),
		Handler: handler,
	}

	shutdownMgr.Register("HTTP Server", 100, func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	c.deps = &AppDependencies{
		HTTPServer: srv,
		Shutdown:   shutdownMgr,
	}

	return nil
}

// Start launches the background runners and waits for context cancellation.
func (c *CompositionRoot) Start(ctx context.Context) error {
	c.mu.Lock()
	deps := c.deps
	c.mu.Unlock()

	if deps == nil {
		return errors.New("app not built, call Build() first")
	}

	serverErr := make(chan error, 1)
	go func() {
		// In real usage this is ListenAndServe; error check ignores ErrServerClosed
		if err := deps.HTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		return deps.Shutdown.Shutdown(context.Background())
	case err := <-serverErr:
		return fmt.Errorf("server crashed: %w", err)
	}
}
