package lifecycle_test

import (
	"context"
	"syscall"
	"testing"
	"time"

	lifecycle "system-design-patterns/patterns/01_lifecycle"
)

func TestSignalController_FirstSignalTriggersShutdown(t *testing.T) {
	ctrl := lifecycle.NewSignalController(1*time.Second, syscall.SIGUSR1)
	defer ctrl.Stop()
	ctrl.Start()

	select {
	case <-ctrl.ShutdownContext().Done():
		t.Fatal("context should not be canceled before signal")
	default:
	}

	ctrl.InjectSignal(syscall.SIGUSR1)

	select {
	case <-ctrl.ShutdownContext().Done():
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected context to be canceled on first signal")
	}
}

func TestSignalController_SecondSignalForcesExit(t *testing.T) {
	ctrl := lifecycle.NewSignalController(1*time.Second, syscall.SIGUSR1)
	defer ctrl.Stop()
	ctrl.Start()

	ctrl.InjectSignal(syscall.SIGUSR1)
	<-ctrl.ShutdownContext().Done()

	select {
	case <-ctrl.ForceExitChannel():
		t.Fatal("force exit channel should not be closed on first signal")
	default:
	}

	// Inject second signal
	ctrl.InjectSignal(syscall.SIGUSR1)

	select {
	case <-ctrl.ForceExitChannel():
		// Success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected force exit channel to be closed on second signal")
	}
}

func TestSignalController_ShutdownDeadlineContext(t *testing.T) {
	ctrl := lifecycle.NewSignalController(50*time.Millisecond, syscall.SIGUSR1)
	defer ctrl.Stop()

	ctx, cancel := ctrl.ShutdownDeadlineContext(context.Background())
	defer cancel()

	select {
	case <-ctx.Done():
		// Should time out around 50ms
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected deadline context to time out")
	}
}
