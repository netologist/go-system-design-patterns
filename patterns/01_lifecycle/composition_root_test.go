package lifecycle_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	lifecycle "system-design-patterns/patterns/01_lifecycle"
)

func TestCompositionRoot_BuildValidation(t *testing.T) {
	root := lifecycle.NewCompositionRoot(lifecycle.AppConfig{
		Port: 0, // invalid
	})

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	err := root.Build(dummyHandler)
	if err == nil {
		t.Fatal("expected build error for invalid port, got nil")
	}
}

func TestCompositionRoot_Lifecycle(t *testing.T) {
	root := lifecycle.NewCompositionRoot(lifecycle.AppConfig{
		Port:            9876,
		ShutdownTimeout: 200 * time.Millisecond,
	})

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	err := root.Build(dummyHandler)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)

	go func() {
		doneCh <- root.Start(ctx)
	}()

	// Allow server to spin up
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	cancel()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("expected clean shutdown, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("composition root shutdown timed out")
	}
}
