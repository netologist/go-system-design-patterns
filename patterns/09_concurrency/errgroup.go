package concurrency

import (
	"context"
	"sync"
)

// Group coordinates a collection of goroutines, canceling their shared context upon the first error.
type Group struct {
	cancel func(error)
	wg     sync.WaitGroup
	errMu  sync.Mutex
	err    error
}

// WithContext returns a new Group and associated Context derived from ctx.
// The derived Context is canceled the first time a function passed to Go returns a non-nil error.
func WithContext(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Group{cancel: cancel}, ctx
}

// Go calls the given function in a new goroutine.
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errMu.Lock()
			if g.err == nil {
				g.err = err
				if g.cancel != nil {
					g.cancel(err)
				}
			}
			g.errMu.Unlock()
		}
	}()
}

// Wait blocks until all goroutines from the Go method have finished, returning the first non-nil error (if any).
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel(nil)
	}
	return g.err
}
