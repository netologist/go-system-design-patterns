package dataintegrity

import (
	"sync"
)

type keyRefMutex struct {
	mu       sync.Mutex
	refCount int
}

// KeyedMutexLock coordinates fine-grained pessimistic locking per resource ID (simulating SELECT FOR UPDATE).
type KeyedMutexLock struct {
	globalMu sync.Mutex
	locks    map[string]*keyRefMutex
}

func NewKeyedMutexLock() *KeyedMutexLock {
	return &KeyedMutexLock{
		locks: make(map[string]*keyRefMutex),
	}
}

// Lock acquires exclusive lock for the specific key.
func (l *KeyedMutexLock) Lock(key string) func() {
	l.globalMu.Lock()
	entry, ok := l.locks[key]
	if !ok {
		entry = &keyRefMutex{}
		l.locks[key] = entry
	}
	entry.refCount++
	l.globalMu.Unlock()

	entry.mu.Lock()

	// Return unlock closure
	return func() {
		entry.mu.Unlock()

		l.globalMu.Lock()
		defer l.globalMu.Unlock()
		entry.refCount--
		if entry.refCount == 0 {
			delete(l.locks, key)
		}
	}
}
