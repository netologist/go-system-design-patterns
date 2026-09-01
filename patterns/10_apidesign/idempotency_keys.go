package apidesign

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"
)

var (
	ErrMissingIdempotencyKey  = errors.New("missing Idempotency-Key header")
	ErrConcurrentIdempotentOp = errors.New("concurrent operation with same Idempotency-Key already in flight")
)

const HeaderIdempotencyKey = "Idempotency-Key"

// CachedResponse stores the recorded status, body, and hash of the original request.
type CachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	PayloadHash string
	CreatedAt  time.Time
}

// MemoryIdempotencyStore stores idempotency keys and their cached responses.
type MemoryIdempotencyStore struct {
	mu    sync.RWMutex
	cache map[string]*CachedResponse
	locks map[string]struct{}
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		cache: make(map[string]*CachedResponse),
		locks: make(map[string]struct{}),
	}
}

// LockKey attempts to lock a key for an in-flight operation.
func (s *MemoryIdempotencyStore) LockKey(key string) (*CachedResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already completed and cached
	if cached, ok := s.cache[key]; ok {
		return cached, false
	}

	// Check if currently locked
	if _, locked := s.locks[key]; locked {
		return nil, false
	}

	s.locks[key] = struct{}{}
	return nil, true
}

// SaveResponse caches the completed response and releases the lock.
func (s *MemoryIdempotencyStore) SaveResponse(key, payloadHash string, status int, header http.Header, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.locks, key)
	s.cache[key] = &CachedResponse{
		StatusCode:  status,
		Header:      header.Clone(),
		Body:        body,
		PayloadHash: payloadHash,
		CreatedAt:   time.Now(),
	}
}

func HashPayload(body []byte) string {
	h := sha256.Sum256(body)
	return hex.EncodeToString(h[:])
}
