package persistence

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var ErrPoolExhausted = errors.New("connection pool capacity exhausted")

// PoolStats holds diagnostic metrics about the pool.
type PoolStats struct {
	MaxCapacity int
	ActiveConns int
	IdleConns   int
	TotalWaits  int64
}

// Connection represents an acquired database connection.
type Connection struct {
	ID        int
	CreatedAt time.Time
	pool      *ConnectionPoolLimiter
	released  atomic.Bool
}

// Close releases the connection back to the pool.
func (c *Connection) Close() {
	if c.released.CompareAndSwap(false, true) {
		c.pool.release(c)
	}
}

// ConnectionPoolLimiter enforces upper bounds on database connections.
type ConnectionPoolLimiter struct {
	maxCapacity int
	tokens      chan struct{}
	idleConns   chan *Connection
	totalWaits  atomic.Int64
	nextID      atomic.Int32
	mu          sync.Mutex
	closed      bool
}

// NewConnectionPoolLimiter creates a bounded connection pool.
func NewConnectionPoolLimiter(maxCapacity int) *ConnectionPoolLimiter {
	if maxCapacity <= 0 {
		maxCapacity = 10
	}

	tokens := make(chan struct{}, maxCapacity)
	for range maxCapacity {
		tokens <- struct{}{}
	}

	return &ConnectionPoolLimiter{
		maxCapacity: maxCapacity,
		tokens:      tokens,
		idleConns:   make(chan *Connection, maxCapacity),
	}
}

// Acquire acquires a connection within context deadline or returns error.
func (p *ConnectionPoolLimiter) Acquire(ctx context.Context) (*Connection, error) {
	// First check if an idle connection is available immediately
	select {
	case conn := <-p.idleConns:
		conn.released.Store(false)
		return conn, nil
	default:
	}

	// Try to acquire a token from bounded pool
	select {
	case <-p.tokens:
		conn := &Connection{
			ID:        int(p.nextID.Add(1)),
			CreatedAt: time.Now(),
			pool:      p,
		}
		return conn, nil
	default:
		// Pool is full, must wait with context
		p.totalWaits.Add(1)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrPoolExhausted, ctx.Err())
		case <-p.tokens:
			conn := &Connection{
				ID:        int(p.nextID.Add(1)),
				CreatedAt: time.Now(),
				pool:      p,
			}
			return conn, nil
		case conn := <-p.idleConns:
			conn.released.Store(false)
			return conn, nil
		}
	}
}

func (p *ConnectionPoolLimiter) release(c *Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}

	select {
	case p.idleConns <- c:
	default:
		// Return token if idle buffer is full
		p.tokens <- struct{}{}
	}
}

// Stats returns current snapshot of pool utilization.
func (p *ConnectionPoolLimiter) Stats() PoolStats {
	idle := len(p.idleConns)
	availableTokens := len(p.tokens)
	active := p.maxCapacity - (idle + availableTokens)
	if active < 0 {
		active = 0
	}

	return PoolStats{
		MaxCapacity: p.maxCapacity,
		ActiveConns: active,
		IdleConns:   idle,
		TotalWaits:  p.totalWaits.Load(),
	}
}
