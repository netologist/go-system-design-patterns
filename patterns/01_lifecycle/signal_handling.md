# Signal Handling Pattern

## 1. Overview & Concept
The **Signal Handling** pattern manages operating system POSIX termination and notification signals (such as `SIGTERM`, `SIGINT`, `SIGHUP`) to safely control the lifecycle of a Go service.

By intercepting OS signals via asynchronous channels and translating them into standard Go `context.Context` cancellations, the pattern provides an idiomatic, non-blocking bridge between the host operating system (systemd, Docker, Kubernetes) and the internal concurrent goroutine ecosystem. It also introduces a "double-signal" safeguard allowing human operators or automation scripts to trigger an immediate, forced abort if graceful shutdown hangs on deadlocked resources.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without robust, centralized signal handling:

- **Default Runtime Termination (`os.Exit(2)`)**: The Go runtime terminates immediately upon receiving an unhandled `SIGTERM` or `SIGINT`. This instantly kills all running goroutines without executing deferred functions (`defer`), corrupting active database transactions and dropping in-flight HTTP requests.
- **Hanging Graceful Shutdowns**: If a graceful shutdown hook deadlocks on a broken network socket or full channel, the process remains stuck indefinitely. Operators are forced to manually issue a destructive `kill -9` (`SIGKILL`), leaving no forensic telemetry.
- **Signal Dropping on Unbuffered Channels**: Using an unbuffered signal channel (`make(chan os.Signal)`) causes `signal.Notify` to drop incoming signals silently if the receiver goroutine is momentarily blocked or busy.
- **Fragmented Signal Handling**: Registering multiple independent `signal.Notify` listeners across different application packages leads to race conditions, unpredictable shutdown ordering, and uncoordinated resource cleanup.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Signal Controller Sequence Diagram

```
Operating System / Terminal           SignalController              ShutdownContext()              ForceExitChannel()
            |                                |                              |                               |
            |--- 1st Signal (SIGTERM/INT) -->|                              |                               |
            |                                |-- Cancel Context ----------->| (Context canceled)            |
            |                                |                              | (Triggers Graceful Shutdown)  |
            |                                |                              |                               |
            |                                |-- Start Deadline Timer       |                               |
            |                                |                                                              |
            |                                +---------------+----------------------------------------------+
            |                                                |
            | [Scenario A: Clean Shutdown]                   | [Scenario B: 2nd Signal Received (Double Ctrl+C)]
            |                                                |
            | Graceful shutdown completes                    |--- 2nd Signal (SIGINT) ------>|
            | Process exits cleanly (code 0)                 |                               |-- Close Channel ->|
            |                                                |                               | (Immediate abort) |
            |                                                |                               +-------------------+
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                           SIGNAL CONTROLLER                             |
 |                                                                         |
 |  Initialization:                                                        |
 |  - Buffer size = 2 (make(chan os.Signal, 2))                            |
 |  - Listens to: syscall.SIGINT, syscall.SIGTERM                          |
 |  - Creates root: shutdownCtx, cancelShutdown                            |
 |  - Creates exit channel: forceExitChan                                  |
 |                                                                         |
 |  Event Loop (Background Goroutine):                                     |
 |  1. Block on sigChan (Waiting for first OS signal)                      |
 |  2. On First Signal -> Execute cancelShutdown()                         |
 |     +--> Propagates cancellation to ShutdownContext()                   |
 |  3. Enter Dual-Wait Select:                                             |
 |     +--> Case A: Second signal on sigChan -> close(forceExitChan)       |
 |     +--> Case B: Deadline expires (time.After) -> Return                |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Always Buffer the Signal Channel**: Allocate signal channels with a capacity of at least 2 (`make(chan os.Signal, 2)`) so `signal.Notify` never drops incoming signals if the runtime loop is temporarily scheduled away.
- **Listen for Both `SIGINT` and `SIGTERM`**: Container orchestrators (Kubernetes, Docker) send `SIGTERM` on shutdown, whereas interactive terminal sessions send `SIGINT` (`Ctrl+C`). Always capture both.
- **Implement Double-Signal Force Kill**: If an operator presses `Ctrl+C` a second time during shutdown, immediately close the `ForceExitChannel()` to break out of hanging cleanup steps and exit without waiting for the timeout.
- **Always Call `signal.Stop()`**: When the controller terminates or in tests, invoke `signal.Stop(sigChan)` to deregister signal handlers and prevent memory leaks.

### Common Pitfalls & Anti-Patterns
- **Unbuffered Signal Channels**: Creating `make(chan os.Signal)`. If a signal arrives when no goroutine is reading, `signal.Notify` drops the signal silently.
- **Ignoring `signal.Stop()` in Tests**: Forgetting to clean up signal channels in tests, causing subsequent test cases to intercept signals intended for the test runner.
- **Scattering `signal.Notify` Across Domain Code**: Trapping signals deep in domain services rather than centralizing signal capture at the Composition Root / `main.go`.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/01_lifecycle/signal_handling.go`.

### Core Types & Signatures

```go
package lifecycle

import (
	"context"
	"os"
	"time"
)

// SignalController listens for OS termination signals and controls the shutdown lifecycle.
type SignalController struct {
	// unexported fields: signals, shutdownCtx, cancelShutdown, deadline, sigChan, once, forceExitChan
}

func NewSignalController(deadline time.Duration, signals ...os.Signal) *SignalController
func (s *SignalController) Start()
func (s *SignalController) Stop()
func (s *SignalController) ShutdownContext() context.Context
func (s *SignalController) ShutdownDeadlineContext(parent context.Context) (context.Context, context.CancelFunc)
func (s *SignalController) ForceExitChannel() <-chan struct{}
func (s *SignalController) InjectSignal(sig os.Signal)
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"syscall"
	"time"

	"patterns/01_lifecycle"
)

func main() {
	// 1. Initialize controller with 10s shutdown deadline for SIGINT & SIGTERM
	sigCtrl := lifecycle.NewSignalController(10*time.Second, syscall.SIGINT, syscall.SIGTERM)
	sigCtrl.Start()
	defer sigCtrl.Stop()

	server := &http.Server{Addr: ":8080"}

	// 2. Start HTTP server in background
	go func() {
		log.Println("Server running on :8080. Send SIGINT (Ctrl+C) or SIGTERM to stop...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server listen error: %v", err)
		}
	}()

	// 3. Monitor for shutdown signal or forced exit
	select {
	case <-sigCtrl.ShutdownContext().Done():
		log.Println("Termination signal received. Starting graceful shutdown...")

		// Derived bounded context for teardown
		shutdownCtx, cancel := sigCtrl.ShutdownDeadlineContext(context.Background())
		defer cancel()

		doneCh := make(chan error, 1)
		go func() {
			doneCh <- server.Shutdown(shutdownCtx)
		}()

		select {
		case err := <-doneCh:
			if err != nil {
				log.Printf("Server shutdown error: %v", err)
			}
			log.Println("Server drained successfully. Exiting.")
		case <-sigCtrl.ForceExitChannel():
			log.Println("Second signal received! Forcing immediate exit.")
			os.Exit(1)
		case <-shutdownCtx.Done():
			log.Println("Shutdown deadline exceeded. Forcing exit.")
			os.Exit(1)
		}
	}
}
```
