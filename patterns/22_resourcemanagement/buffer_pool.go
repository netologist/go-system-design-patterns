package resourcemanagement

import (
	"bytes"
	"sync"
)

// SafeBufferPool provides high-performance byte buffer reuse with maximum capacity ceiling to prevent heap bloat.
type SafeBufferPool struct {
	pool        sync.Pool
	maxCapacity int
}

func NewSafeBufferPool(initialCapacity, maxCapacity int) *SafeBufferPool {
	if initialCapacity <= 0 {
		initialCapacity = 1024 // 1 KB
	}
	if maxCapacity <= 0 {
		maxCapacity = 64 * 1024 // 64 KB max retention in pool
	}

	return &SafeBufferPool{
		maxCapacity: maxCapacity,
		pool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 0, initialCapacity))
			},
		},
	}
}

// Get acquires a clean buffer from the pool.
func (p *SafeBufferPool) Get() *bytes.Buffer {
	buf := p.pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// Put returns the buffer to the pool only if its capacity does not exceed the safe ceiling.
func (p *SafeBufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}

	// Discard oversized buffers to allow Garbage Collector to reclaim large memory blocks
	if buf.Cap() > p.maxCapacity {
		return
	}

	buf.Reset()
	p.pool.Put(buf)
}
