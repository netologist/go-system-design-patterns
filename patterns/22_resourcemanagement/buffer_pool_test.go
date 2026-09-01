package resourcemanagement_test

import (
	"strings"
	"testing"

	resourcemanagement "system-design-patterns/patterns/22_resourcemanagement"
)

func TestSafeBufferPool_ReuseAndOversizeCeiling(t *testing.T) {
	pool := resourcemanagement.NewSafeBufferPool(128, 512) // 512 bytes ceiling

	// 1. Get and use buffer
	buf := pool.Get()
	buf.WriteString("hello buffer")
	if buf.String() != "hello buffer" {
		t.Errorf("buffer content mismatch")
	}

	// 2. Put back and get again -> should be reset
	pool.Put(buf)
	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Errorf("expected buffer reset on get, got len: %d", buf2.Len())
	}

	// 3. Grow buffer beyond 512 bytes
	buf2.WriteString(strings.Repeat("X", 1000))
	if buf2.Cap() <= 512 {
		t.Fatalf("expected buffer cap > 512, got: %d", buf2.Cap())
	}

	// Putting oversized buffer should safely discard it without panic
	pool.Put(buf2)
}
