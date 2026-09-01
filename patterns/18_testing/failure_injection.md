# Failure Injection & Chaos Testing Pattern

## 1. Overview & Concept
Failure Injection (Chaos Engineering at the application layer) is a pattern where artificial delays, transient network faults, connection resets, and internal server errors are deliberately injected into dependencies during automated tests or staging environments.

## 2. Production Problem & Failure Modes
1. **Untested Resilience Paths**: Circuit breakers, retries, fallbacks, and timeout handlers are rarely exercised during happy-path tests, meaning they fail when real outages occur in production.
2. **Hidden Deadlocks and Leaks**: Timeout and cancellation recovery paths often leak goroutines, channels, or database connections because they were never stressed under real artificial latency.
3. **Cascading Failure Vulnerability**: Without chaos testing, a slow downstream service silently degrades the entire upstream fleet before operators notice.

## 3. Architecture & Mechanism

```text
Caller ---> [ Fault Injector Decorator ] ---> Real Dependency
                      |
                      +---> Inject Latency (e.g. 200ms)
                      +---> Inject Error (e.g. 503 with 50% probability)
                      +---> Pass through on 0% fault
```

## 4. Production Hardening & Trade-offs
- **Deterministic Control**: The fault injector should support exact failure rates (`0.0` to `1.0`), configurable error types, and artificial delays.
- **Context Honoring**: Artificial delays must respect context cancellation (`ctx.Done()`) to test whether caller timeouts abort early as designed.
- **Safety**: Ensure fault injection capabilities are strictly controlled and cannot accidentally run in production without explicit configuration flags.

## 5. Code Walkthrough & Usage
See `failure_injection.go` and `failure_injection_test.go`:
- `FaultInjector`: Injects `FailureRate`, `ExtraDelay`, and `CustomError` to stress-test error and timeout paths.
