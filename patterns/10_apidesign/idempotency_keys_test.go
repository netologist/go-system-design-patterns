package apidesign_test

import (
	"net/http"
	"testing"

	apidesign "system-design-patterns/patterns/10_apidesign"
)

func TestMemoryIdempotencyStore_Replay(t *testing.T) {
	store := apidesign.NewMemoryIdempotencyStore()
	key := "idemp-payment-001"
	payloadHash := apidesign.HashPayload([]byte(`{"amount": 5000}`))

	// 1. Initial attempt gets lock
	cached, locked := store.LockKey(key)
	if !locked || cached != nil {
		t.Fatal("expected key to be locked on first attempt")
	}

	// Save response
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	store.SaveResponse(key, payloadHash, http.StatusCreated, header, []byte(`{"id":"pay_123","status":"paid"}`))

	// 2. Replay with same key -> Returns cached response directly
	cached, locked = store.LockKey(key)
	if locked || cached == nil {
		t.Fatal("expected cached response on replay attempt")
	}

	if cached.StatusCode != http.StatusCreated {
		t.Errorf("expected cached status 201, got: %d", cached.StatusCode)
	}

	if string(cached.Body) != `{"id":"pay_123","status":"paid"}` {
		t.Errorf("cached body mismatch: %s", string(cached.Body))
	}
}
