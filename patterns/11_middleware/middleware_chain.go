package middleware

import "net/http"

// Middleware defines standard HTTP middleware signature.
type Middleware func(http.Handler) http.Handler

// Chain coordinates sequential execution of multiple middlewares (Onion Architecture).
type Chain struct {
	middlewares []Middleware
}

// NewChain creates an empty middleware chain.
func NewChain(middlewares ...Middleware) Chain {
	return Chain{
		middlewares: append(([]Middleware)(nil), middlewares...),
	}
}

// Append creates a new chain with added middlewares.
func (c Chain) Append(middlewares ...Middleware) Chain {
	newMiddlewares := make([]Middleware, 0, len(c.middlewares)+len(middlewares))
	newMiddlewares = append(newMiddlewares, c.middlewares...)
	newMiddlewares = append(newMiddlewares, middlewares...)
	return Chain{middlewares: newMiddlewares}
}

// Then wraps the final http.Handler with all middlewares in outer-to-inner order.
func (c Chain) Then(finalHandler http.Handler) http.Handler {
	if finalHandler == nil {
		finalHandler = http.DefaultServeMux
	}

	for i := len(c.middlewares) - 1; i >= 0; i-- {
		finalHandler = c.middlewares[i](finalHandler)
	}

	return finalHandler
}
