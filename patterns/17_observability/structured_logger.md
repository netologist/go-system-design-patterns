# Structured Logger Pattern (with Context Correlation & Redaction)

## Overview & Definition
The **Structured Logger Pattern** emits log records as structured, machine-parseable data objects (typically JSON or logfmt) rather than unstructured arbitrary text strings. In modern Go applications (accelerated by the standard library's `log/slog` package in Go 1.21+), structured logging transforms logs into queryable streams for centralized telemetry platforms (e.g., Datadog, Grafana Loki, ElasticSearch, AWS CloudWatch).

Beyond simple key-value pairs, a production-grade Structured Logger enforces:
1. **Context Metadata Propagation**: Automatically binding `request_id`, `trace_id`, and `correlation_id` across goroutines.
2. **Automated Secret Masking / Redaction**: Scrubbing sensitive keys (e.g., passwords, authorization tokens, credit cards, SSNs) before serialization.
3. **Dynamic Level Filtering & Sampling**: Filtering low-severity logs (DEBUG/INFO) during high load while always retaining 100% of WARN and ERROR entries.

---

## Problem Statement (Failure Scenarios Without This Pattern)

### 1. Unparseable String Logs in Distributed Incidents
When an engineer logs via `log.Printf("User %s error: %v", userID, err)`:
- Centralized log ingestion engines cannot index fields individually.
- Searching for all errors related to `userID=101` requires slow, expensive full-text regex scans across terabytes of data.
- Tracing a single user request across 10 microservices is impossible without standard correlation IDs in the JSON envelope.

### 2. Accidental PII and Credential Leakage in Logs
A developer logs an incoming HTTP payload (`logger.Info("Login attempt", fields)`). If `fields` contains `"password": "SuperSecret123"`, plain text credentials are permanently written to log aggregators and compliance monitoring systems, causing severe security compliance violations (GDPR, PCI-DSS, SOC2).

### 3. Log Volume Flooding and Disk I/O Starvation
At 50,000 requests per second, logging every single INFO line generates tens of gigabytes of disk I/O per minute, starving application disk throughput and generating astronomical cloud logging bills.

---

## Architectural Mechanism & Flow

```mermaid
flowchart TD
    LogCall[logger.Log ctx, Level, Msg, Fields] --> LevelCheck{Level >= minLevel?}
    LevelCheck -- No --> DropLog([Drop / No-Op])
    
    LevelCheck -- Yes --> SampleCheck{Is DEBUG/INFO &<br/>SampleRate < 1.0?}
    SampleCheck -- Sampled Out (rand > rate) --> DropLog
    SampleCheck -- Sampled In (or WARN/ERROR) --> Redactor[Sanitize Fields: Check Sensitive Keys]
    
    Redactor --> Mask[Redact matches with '[REDACTED]']
    Mask --> BuildRec[Construct LogRecord JSON Envelope<br/>- Timestamp RFC3339Nano<br/>- Level<br/>- Message<br/>- Context IDs<br/>- Fields]
    BuildRec --> Encode[JSON Encoder Stream to io.Writer]
```

---

## Production Best Practices & Pitfalls

### Best Practices
- **JSON Encoding to `io.Writer`**: Stream logs directly to standard output (`os.Stdout`) in containerized environments (Kubernetes/Docker) using a synchronized `json.Encoder`.
- **Always Preserve Errors and Warnings**: Never sample out `WARN` or `ERROR` log levels; sampling should only apply to high-volume `INFO` and `DEBUG` events.
- **Automated Case-Insensitive Key Redaction**: Match against sensitive substring patterns (`"token"`, `"auth"`, `"password"`, `"secret"`, `"credit_card"`) regardless of casing (`"Authorization"`, `"authToken"`).
- **Go 1.21+ `log/slog` Integration**: Utilize Go's native `log/slog.Handler` interface for zero-allocation structured logging pipelines.

### Pitfalls to Avoid
- **Logging Inside Critical Hot Loops**: Avoid logging millions of items inside tight data transformation loops; aggregate metrics or summarize instead.
- **Mutating Shared Field Maps**: Always allocate a clean sanitized map when redacting to avoid data race collisions with other concurrent goroutines.

---

## Code Walkthrough & Usage

In `patterns/17_observability/structured_logger.go`, `StructuredLogger` implements level filtering, probabilistic sampling, and sensitive field masking:

```go
type LogLevel string

const (
    LevelDebug LogLevel = "DEBUG"
    LevelInfo  LogLevel = "INFO"
    LevelWarn  LogLevel = "WARN"
    LevelError LogLevel = "ERROR"
)

type LogRecord struct {
    Timestamp     string         `json:"timestamp"`
    Level         LogLevel       `json:"level"`
    Message       string         `json:"message"`
    RequestID     string         `json:"request_id,omitempty"`
    CorrelationID string         `json:"correlation_id,omitempty"`
    Fields        map[string]any `json:"fields,omitempty"`
}

type StructuredLogger struct {
    mu            sync.Mutex
    out           io.Writer
    sampleRate    float64
    minLevel      LogLevel
    sensitiveKeys []string
}
```

### Logging with Level Filtering & Redaction
```go
func (l *StructuredLogger) Log(ctx context.Context, level LogLevel, msg string, fields map[string]any) {
    // 1. Min Level Check
    if !l.shouldLogLevel(level) {
        return
    }

    // 2. Sampling: Always keep WARN/ERROR; probabilistically sample DEBUG/INFO
    if level == LevelDebug || level == LevelInfo {
        if l.sampleRate < 1.0 && rand.Float64() > l.sampleRate {
            return // Sampled out to save I/O
        }
    }

    // 3. Automated PII & Credential Sanitization
    sanitizedFields := l.sanitizeFields(fields)

    rec := LogRecord{
        Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
        Level:     level,
        Message:   msg,
        Fields:    sanitizedFields,
    }

    l.mu.Lock()
    defer l.mu.Unlock()
    _ = json.NewEncoder(l.out).Encode(rec)
}
```

### Sanitization Implementation
```go
func (l *StructuredLogger) sanitizeFields(fields map[string]any) map[string]any {
    if fields == nil {
        return nil
    }

    out := make(map[string]any, len(fields))
    for k, v := range fields {
        kLower := strings.ToLower(k)
        isSensitive := false
        for _, s := range l.sensitiveKeys {
            if strings.Contains(kLower, s) {
                isSensitive = true
                break
            }
        }
        if isSensitive {
            out[k] = "[REDACTED]"
        } else {
            out[k] = v
        }
    }
    return out
}
```
