package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// SignalController listens for OS termination signals and controls the shutdown lifecycle.
type SignalController struct {
	signals        []os.Signal
	shutdownCtx    context.Context
	cancelShutdown context.CancelFunc
	deadline       time.Duration
	sigChan        chan os.Signal
	once           sync.Once
	forceExitChan  chan struct{}
}

// NewSignalController creates a signal controller with default SIGINT and SIGTERM listeners.
func NewSignalController(deadline time.Duration, signals ...os.Signal) *SignalController {
	if len(signals) == 0 {
		signals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SignalController{
		signals:        signals,
		shutdownCtx:    ctx,
		cancelShutdown: cancel,
		deadline:       deadline,
		sigChan:        make(chan os.Signal, 2),
		forceExitChan:  make(chan struct{}),
	}
}

// Start begins listening for signals in the background.
func (s *SignalController) Start() {
	s.once.Do(func() {
		signal.Notify(s.sigChan, s.signals...)
		go s.loop()
	})
}

// Stop stops listening for signals.
func (s *SignalController) Stop() {
	signal.Stop(s.sigChan)
}

func (s *SignalController) loop() {
	firstSig := <-s.sigChan
	if firstSig == nil {
		return
	}

	// Trigger graceful shutdown
	s.cancelShutdown()

	// Listen for a second signal to force immediate exit
	select {
	case secondSig := <-s.sigChan:
		if secondSig != nil {
			close(s.forceExitChan)
		}
	case <-time.After(s.deadline):
		// Deadline elapsed
	}
}

// ShutdownContext returns a context canceled on first signal.
func (s *SignalController) ShutdownContext() context.Context {
	return s.shutdownCtx
}

// ShutdownDeadlineContext returns a context with timeout starting when shutdown is triggered.
func (s *SignalController) ShutdownDeadlineContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.deadline)
}

// ForceExitChannel returns a channel closed if a second signal is received during shutdown.
func (s *SignalController) ForceExitChannel() <-chan struct{} {
	return s.forceExitChan
}

// InjectSignal manually simulates receiving an OS signal (for tests / manual trigger).
func (s *SignalController) InjectSignal(sig os.Signal) {
	s.sigChan <- sig
}
