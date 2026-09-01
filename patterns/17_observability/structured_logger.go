package observability

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

// LogRecord structured JSON log line.
type LogRecord struct {
	Timestamp     string         `json:"timestamp"`
	Level         LogLevel       `json:"level"`
	Message       string         `json:"message"`
	RequestID     string         `json:"request_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

// StructuredLogger produces JSON logs with context metadata, secret masking, and sampling.
type StructuredLogger struct {
	mu           sync.Mutex
	out          io.Writer
	sampleRate   float64 // 0.0 to 1.0 (e.g. 1.0 logs 100%, 0.1 logs 10%)
	minLevel     LogLevel
	sensitiveKeys []string
}

func NewStructuredLogger(out io.Writer, sampleRate float64, minLevel LogLevel) *StructuredLogger {
	if sampleRate <= 0 || sampleRate > 1.0 {
		sampleRate = 1.0
	}

	return &StructuredLogger{
		out:        out,
		sampleRate: sampleRate,
		minLevel:   minLevel,
		sensitiveKeys: []string{
			"password", "secret", "token", "key", "auth", "credential", "credit_card", "ssn",
		},
	}
}

func (l *StructuredLogger) Log(ctx context.Context, level LogLevel, msg string, fields map[string]any) {
	// Level filtering
	if !l.shouldLogLevel(level) {
		return
	}

	// Sampling (Always log WARN and ERROR, sample DEBUG and INFO)
	if level == LevelDebug || level == LevelInfo {
		if l.sampleRate < 1.0 && rand.Float64() > l.sampleRate {
			return // Sampled out
		}
	}

	// Redact sensitive keys
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

func (l *StructuredLogger) shouldLogLevel(level LogLevel) bool {
	order := map[LogLevel]int{
		LevelDebug: 1,
		LevelInfo:  2,
		LevelWarn:  3,
		LevelError: 4,
	}
	return order[level] >= order[l.minLevel]
}
