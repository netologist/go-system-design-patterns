package caching_test

import (
	"context"
	"testing"
	"time"

	caching "system-design-patterns/patterns/14_caching"
)

func TestCacheAsideService_ReadAndInvalidate(t *testing.T) {
	cache := caching.NewMemoryCache[string]()
	ctx := context.Background()

	dbCalls := 0
	dbData := map[string]string{"user:1": "Initial Name"}

	dbGet := func(c context.Context, key string) (string, error) {
		dbCalls++
		return dbData[key], nil
	}
	dbSet := func(c context.Context, key string, val string) error {
		dbData[key] = val
		return nil
	}

	service := caching.NewCacheAsideService[string](cache, 1*time.Hour, dbGet, dbSet)

	// 1st Read: Cache miss -> DB fetch (dbCalls = 1)
	val, err := service.Get(ctx, "user:1")
	if err != nil || val != "Initial Name" {
		t.Fatalf("first read failed: %v", err)
	}
	if dbCalls != 1 {
		t.Errorf("expected 1 DB call on first read, got: %d", dbCalls)
	}

	// 2nd Read: Cache hit -> No DB fetch (dbCalls still 1)
	val, err = service.Get(ctx, "user:1")
	if err != nil || val != "Initial Name" {
		t.Fatalf("second read failed: %v", err)
	}
	if dbCalls != 1 {
		t.Errorf("expected 0 additional DB calls on cache hit, got total: %d", dbCalls)
	}

	// Mutation: Updates DB and invalidates cache
	err = service.Set(ctx, "user:1", "Updated Name")
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// 3rd Read: Cache was invalidated -> Fetches updated DB data (dbCalls = 2)
	val, err = service.Get(ctx, "user:1")
	if err != nil || val != "Updated Name" {
		t.Fatalf("third read failed: %v", err)
	}
	if dbCalls != 2 {
		t.Errorf("expected 2 DB calls after invalidation, got: %d", dbCalls)
	}
}
