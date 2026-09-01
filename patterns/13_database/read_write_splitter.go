package database

import (
	"context"
	"errors"
	"sync"
	"time"
)

type DBEndpointType string

const (
	EndpointPrimary DBEndpointType = "PRIMARY"
	EndpointReplica DBEndpointType = "REPLICA"
)

type recentWriteCtxKey struct{}

// WithRecentWrite marks the context as having recently written data, forcing subsequent reads to Primary DB.
func WithRecentWrite(ctx context.Context, duration time.Duration) context.Context {
	expiry := time.Now().Add(duration)
	return context.WithValue(ctx, recentWriteCtxKey{}, expiry)
}

func hasRecentWrite(ctx context.Context) bool {
	if val, ok := ctx.Value(recentWriteCtxKey{}).(time.Time); ok {
		return time.Now().Before(val)
	}
	return false
}

// DBExecutor simulates executing a SQL query.
type DBExecutor interface {
	Exec(ctx context.Context, query string) error
	Query(ctx context.Context, query string) (string, error)
}

// ReadWriteRouter routes writes to primary and reads to replica, except when replica lag awareness applies.
type ReadWriteRouter struct {
	primary DBExecutor
	replica DBExecutor
	mu      sync.RWMutex
}

func NewReadWriteRouter(primary, replica DBExecutor) *ReadWriteRouter {
	return &ReadWriteRouter{
		primary: primary,
		replica: replica,
	}
}

// Write always routes to Primary.
func (r *ReadWriteRouter) Write(ctx context.Context, query string) error {
	if r.primary == nil {
		return errors.New("primary DB unavailable")
	}
	return r.primary.Exec(ctx, query)
}

// Read routes to Replica by default, or Primary if recent write occurred in session.
func (r *ReadWriteRouter) Read(ctx context.Context, query string) (endpoint DBEndpointType, result string, err error) {
	if hasRecentWrite(ctx) {
		// Read-your-own-writes consistency: route to Primary to avoid replica lag
		res, err := r.primary.Query(ctx, query)
		return EndpointPrimary, res, err
	}

	// Normal read: route to Replica
	res, err := r.replica.Query(ctx, query)
	return EndpointReplica, res, err
}
