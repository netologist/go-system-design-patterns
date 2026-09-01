# Composition Root Pattern

## 1. Overview & Concept
The **Composition Root** is the single, centralized location at the entry point of an application (typically `main.go` or an explicit bootstrap module) where the entire dependency graph is wired together and the application lifecycle is managed.

Instead of allowing domain entities, repositories, or services to instantiate their own dependencies, read environment variables directly, or manage their own long-running threads, the Composition Root acts as the master coordinator:
1. It loads and validates configuration.
2. It instantiates infrastructure drivers (databases, cache clients, loggers).
3. It constructs domain repositories and injects them into business services.
4. It attaches business services to transport layer handlers (HTTP, gRPC, event consumers).
5. It registers lifecycle cleanup hooks and starts server listeners.

---

## 2. Production Problem & Failure Modes (Why do we need this in high-scale backends?)

Without a dedicated Composition Root, applications suffer from severe architectural decay:

- **Scattered Service Instantiation & Hidden Dependencies**: Packages randomly call `sql.Open()`, initialize Redis singletons, or invoke `os.Getenv()` inside deep business logic functions, creating hidden coupling that is impossible to unit test.
- **Service Locator Anti-Patterns & Global State**: Relying on global variables (e.g., `var DB *sql.DB`, `var AppConfig Config`) creates data races during testing and prevents running test suites in parallel with `t.Parallel()`.
- **Cyclic Dependency Nightmares**: When domain packages instantiate infrastructure packages that also need domain entities, Go's compiler rejects the circular package import dependencies.
- **Uncontrolled Goroutine Lifecycles**: Launching unmanaged background goroutines deep inside domain services leads to orphaned threads, memory leaks, and unpredictable application shutdowns.

---

## 3. Architecture & Mechanism (Text/ASCII diagram or sequence)

### Dependency Wiring Hierarchy (ASCII)

```
 +-------------------------------------------------------------------------+
 |                          COMPOSITION ROOT (main)                        |
 |                                                                         |
 |  1. Config: Typed Config & Secret Sanitization                          |
 |  2. Drivers: PostgreSQL Connection Pool (*sql.DB), Redis Client         |
 |  3. Repositories: PostgresUserRepository (*sql.DB)                      |
 |  4. Services: UserService(UserRepo, Notifier)                           |
 |  5. Handlers: HTTPUserHandler(UserService)                              |
 |  6. Lifecycle: GracefulShutdownManager & SignalController               |
 +-------------------------------------------------------------------------+
                                    |
            +-----------------------+-----------------------+
            |                                               |
            v                                               v
 +----------------------+                       +----------------------+
 |    TRANSPORT LAYER   |                       |    LIFECYCLE ROOT    |
 |  HTTP / gRPC Routers |                       |  Signal Trap & Drain |
 +----------------------+                       +----------------------+
            |
            v
 +----------------------+
 |     DOMAIN LAYER     |
 |  Business Services   |
 +----------------------+
            |
            v
 +----------------------+
 | INFRASTRUCTURE LAYER |
 | DB / Cache / Clients |
 +----------------------+
```

### Architectural State Machine (ASCII)

```
 +-------------------------------------------------------------------------+
 |                         COMPOSITION ROOT STATE                          |
 |                                                                         |
 |  Phase 1: Bootstrapping (NewCompositionRoot)                            |
 |  - Stores immutable AppConfig                                           |
 |                                                                         |
 |  Phase 2: Wiring (Build(handler))                                       |
 |  - Validates configuration parameters (e.g., Port > 0)                  |
 |  - Instantiates GracefulShutdownManager with configured timeout         |
 |  - Instantiates & configures http.Server                                |
 |  - Registers shutdown hook for HTTP server                              |
 |  - Populates AppDependencies struct                                     |
 |                                                                         |
 |  Phase 3: Execution (Start(ctx))                                        |
 |  - Spawns background goroutine running srv.ListenAndServe()             |
 |  - Ignores http.ErrServerClosed on shutdown                             |
 |  - Blocks until ctx.Done() or fatal server crash                        |
 |  - On ctx.Done() -> Triggers GracefulShutdownManager.Shutdown()         |
 +-------------------------------------------------------------------------+
```

---

## 4. Production Hardening & Trade-offs (Edge cases, pitfalls, performance considerations)

### Production Hardening Checklist
- **Assemble as Close to `main` as Possible**: The Composition Root belongs at the top layer of your application hierarchy (e.g., `cmd/server/main.go` or `internal/bootstrap/root.go`).
- **No Business Logic in the Root**: Keep the Composition Root purely structural—it should only instantiate, wire, and execute lifecycle commands.
- **Check for `http.ErrServerClosed`**: When listening on HTTP servers, ignore `http.ErrServerClosed` on return, as it indicates a normal graceful shutdown rather than a crash.
- **Enforce Inward Dependency Flow**: Dependencies flow strictly inward from the Composition Root to the domain. Domain packages must never import bootstrap or root packages.

### Common Pitfalls & Anti-Patterns
- **Domain Packages Importing Composition Root**: Allowing domain services to reference the bootstrap package creates circular dependencies and tight coupling.
- **Double Calling `Build()`**: Modifying dependencies after the application has already started accepting concurrent requests introduces data races.
- **Silently Swallowing Fatal Errors**: Ignoring errors during dependency instantiation causes services to start in a zombie state with nil pointers.

---

## 5. Implementation Reference & Code Usage (Grounded in the repository's Go code)

The pattern is implemented in `patterns/01_lifecycle/composition_root.go`.

### Core Types & Signatures

```go
package lifecycle

import (
	"context"
	"net/http"
	"time"
)

// AppConfig represents bootstrap configuration.
type AppConfig struct {
	Port            int
	ShutdownTimeout time.Duration
}

// AppDependencies represents instantiated infrastructure and domain components.
type AppDependencies struct {
	HTTPServer *http.Server
	Shutdown   *GracefulShutdownManager
}

// CompositionRoot builds and runs the application lifecycle.
type CompositionRoot struct {
	// unexported fields: config, deps, mu
}

func NewCompositionRoot(cfg AppConfig) *CompositionRoot
func (c *CompositionRoot) Build(handler http.Handler) error
func (c *CompositionRoot) Start(ctx context.Context) error
```

### Complete End-to-End Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"patterns/01_lifecycle"
)

func main() {
	// 1. Prepare configuration
	cfg := lifecycle.AppConfig{
		Port:            8080,
		ShutdownTimeout: 5 * time.Second,
	}

	// 2. Instantiate composition root
	root := lifecycle.NewCompositionRoot(cfg)

	// 3. Define HTTP routes & business handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	// 4. Build application dependency graph
	if err := root.Build(mux); err != nil {
		log.Fatalf("Failed to build application: %v", err)
	}

	// 5. Setup termination signal context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Starting application on port %d...", cfg.Port)
	if err := root.Start(ctx); err != nil {
		log.Fatalf("Application terminated with error: %v", err)
	}
	log.Println("Application stopped cleanly.")
}
```
