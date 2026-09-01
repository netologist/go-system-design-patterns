package resourcemanagement_test

import (
	"context"
	"testing"
	"time"

	resourcemanagement "system-design-patterns/patterns/22_resourcemanagement"
)

type dummyCloser struct {
	closed bool
}

func (d *dummyCloser) Close() error {
	d.closed = true
	return nil
}

func TestResourceTracker_Lifecycle(t *testing.T) {
	tracker := resourcemanagement.NewResourceTracker(2) // Max 2

	c1 := &dummyCloser{}
	c2 := &dummyCloser{}
	c3 := &dummyCloser{}

	_ = tracker.Track("res-1", c1, 5*time.Second)
	_ = tracker.Track("res-2", c2, 5*time.Second)

	// 3rd track exceeds upper bound
	err := tracker.Track("res-3", c3, 5*time.Second)
	if err == nil {
		t.Fatal("expected error on exceeding max resource capacity")
	}

	// Release res-1
	_ = tracker.Release("res-1")
	if !c1.closed {
		t.Errorf("expected c1 closed on release")
	}

	// CloseAll closes remaining res-2
	_ = tracker.CloseAll(context.Background())
	if !c2.closed {
		t.Errorf("expected c2 closed on CloseAll")
	}
	if tracker.ActiveCount() != 0 {
		t.Errorf("expected active count 0, got: %d", tracker.ActiveCount())
	}
}
