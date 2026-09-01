package config

import (
	"encoding/json"
	"strings"
)

const RedactedPlaceholder = "[REDACTED]"

// Secret wraps a sensitive value (API key, password, token) to prevent accidental logging.
type Secret[T any] struct {
	value T
}

// NewSecret wraps a sensitive value into a Secret container.
func NewSecret[T any](val T) Secret[T] {
	return Secret[T]{value: val}
}

// Expose returns the underlying secret value. Use only when passing to authenticators.
func (s Secret[T]) Expose() T {
	return s.value
}

// String implements fmt.Stringer to ensure %s prints [REDACTED].
func (s Secret[T]) String() string {
	return RedactedPlaceholder
}

// GoString implements fmt.GoStringer to ensure %#v prints [REDACTED].
func (s Secret[T]) GoString() string {
	return RedactedPlaceholder
}

// MarshalJSON implements json.Marshaler to ensure json.Marshal hides the secret.
func (s Secret[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(RedactedPlaceholder)
}

// SanitizeMap masks known sensitive keys in a map (useful for logging config maps).
func SanitizeMap(input map[string]string) map[string]string {
	sensitiveKeywords := []string{
		"pass", "secret", "token", "key", "auth", "credential", "private", "cert",
	}

	sanitized := make(map[string]string, len(input))
	for k, v := range input {
		lowerKey := strings.ToLower(k)
		isSensitive := false
		for _, kw := range sensitiveKeywords {
			if strings.Contains(lowerKey, kw) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			sanitized[k] = RedactedPlaceholder
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}

// RedactURL removes user credentials from connection strings (e.g. postgres://user:pass@host/db).
func RedactURL(rawURL string) string {
	idxAt := strings.Index(rawURL, "@")
	if idxAt == -1 {
		return rawURL
	}
	idxScheme := strings.Index(rawURL, "://")
	if idxScheme == -1 || idxScheme > idxAt {
		return rawURL
	}
	return rawURL[:idxScheme+3] + "user:****" + rawURL[idxAt:]
}
