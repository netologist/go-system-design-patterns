package caching_test

import (
	"testing"
	"time"

	caching "system-design-patterns/patterns/14_caching"
)

func TestInvalidationManager_PrefixAndTag(t *testing.T) {
	mgr := caching.NewInvalidationManager()

	mgr.Set("user:1:profile", "Profile1", 1*time.Hour, "tenant:100")
	mgr.Set("user:1:settings", "Settings1", 1*time.Hour, "tenant:100")
	mgr.Set("user:2:profile", "Profile2", 1*time.Hour, "tenant:200")

	// Invalidate by prefix "user:1:"
	dropped := mgr.InvalidatePrefix("user:1:")
	if dropped != 2 {
		t.Errorf("expected 2 keys dropped by prefix, got: %d", dropped)
	}

	if _, ok := mgr.Get("user:1:profile"); ok {
		t.Error("expected user:1:profile to be invalidated")
	}
	if _, ok := mgr.Get("user:2:profile"); !ok {
		t.Error("expected user:2:profile to remain valid")
	}

	// Invalidate by tag "tenant:200"
	dropped = mgr.InvalidateTag("tenant:200")
	if dropped != 1 {
		t.Errorf("expected 1 key dropped by tag, got: %d", dropped)
	}
	if _, ok := mgr.Get("user:2:profile"); ok {
		t.Error("expected user:2:profile to be invalidated by tag")
	}
}
