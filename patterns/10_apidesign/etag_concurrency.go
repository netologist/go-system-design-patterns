package apidesign

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// GenerateETag computes a strong ETag from byte payload.
func GenerateETag(payload []byte) string {
	h := sha256.Sum256(payload)
	return fmt.Sprintf(`"%s"`, hex.EncodeToString(h[:16]))
}

// CheckIfNoneMatch compares the client's If-None-Match header with current ETag.
// Returns true if matched (meaning resource has NOT changed -> 304 Not Modified).
func CheckIfNoneMatch(r *http.Request, currentETag string) bool {
	clientETag := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if clientETag == "" {
		return false
	}

	if clientETag == "*" {
		return true
	}

	for _, tag := range strings.Split(clientETag, ",") {
		if strings.TrimSpace(tag) == currentETag {
			return true
		}
	}

	return false
}

// CheckIfMatch compares the client's If-Match header for optimistic updates.
// Returns true if valid to proceed, false if precondition failed (HTTP 412).
func CheckIfMatch(r *http.Request, currentETag string) bool {
	clientETag := strings.TrimSpace(r.Header.Get("If-Match"))
	// If no header provided, allow update (or enforce if strict mode)
	if clientETag == "" {
		return true
	}

	if clientETag == "*" {
		return true
	}

	for _, tag := range strings.Split(clientETag, ",") {
		if strings.TrimSpace(tag) == currentETag {
			return true
		}
	}

	return false
}
