# ETag Generation and Optimistic Concurrency Control

## Overview & Definition

In distributed REST APIs, two common challenges are:
1. **Bandwidth Optimization via Caching:** Preventing clients from re-downloading unchanged resource payloads (`GET` $\rightarrow$ `304 Not Modified`).
2. **Lost Update Prevention (Optimistic Concurrency):** Preventing two concurrent clients from editing the same resource simultaneously and overwriting each other's changes (`PUT/PATCH` $\rightarrow$ `412 Precondition Failed`).

The **ETag Generation and Optimistic Concurrency Control** pattern implements HTTP/1.1 RFC 7232 / RFC 9110 conditional request validation:
* `GenerateETag(payload)`: Computes a strong, quoted entity tag derived from a cryptographic SHA-256 hash.
* `CheckIfNoneMatch(r, currentETag)`: Evaluates `If-None-Match` headers for read caching.
* `CheckIfMatch(r, currentETag)`: Evaluates `If-Match` headers for safe concurrent updates.

---

## Problem Statement

Without conditional headers and ETags:

* **The "Lost Update" Problem:**
  * User A reads Order #100 (Status: "Pending").
  * User B reads Order #100 (Status: "Pending").
  * User A updates Status to "Approved".
  * User B updates Status to "Canceled".
  * User B's write silently overwrites User A's update without either user knowing a conflict occurred.
* **Wasteful Bandwidth & Serialization Overhead:** Mobile clients querying polling endpoints repeatedly download identical multi-megabyte JSON payloads even when data hasn't changed.

---

## Architectural Mechanism & Flow

```
+-----------------------------------------------------------------------------------+
| 1. READ CONDITIONAL CACHING (GET /resource)                                       |
|                                                                                   |
|  Client (Sends: If-None-Match: "3a4b5c...")                                       |
|     |                                                                             |
|     v                                                                             |
|  Server calculates current ETag: "3a4b5c..."                                      |
|     |                                                                             |
|     v                                                                             |
|  CheckIfNoneMatch == true -> Return HTTP 304 Not Modified (0-byte body)           |
+-----------------------------------------------------------------------------------+

+-----------------------------------------------------------------------------------+
| 2. WRITE OPTIMISTIC CONCURRENCY (PUT /resource)                                   |
|                                                                                   |
|  Client (Sends: If-Match: "3a4b5c...")                                            |
|     |                                                                             |
|     v                                                                             |
|  Server loads resource -> current ETag: "9f8e7d..." (Modified by another user!)    |
|     |                                                                             |
|     v                                                                             |
|  CheckIfMatch == false -> Return HTTP 412 Precondition Failed                     |
|  (Aborts update, prevents lost update bug)                                        |
+-----------------------------------------------------------------------------------+
```

---

## Production Best Practices & Pitfalls

### Best Practices
* **Quote ETags According to RFC Spec:** Always wrap ETags in double quotes (e.g. `"a1b2c3d4"`), as required by HTTP RFC standards.
* **Derive ETags from Resource Versions or Hashes:** If an entity has a monotonic `version` or `updated_at` timestamp in the database (e.g. `fmt.Sprintf("\"v%d-%d\"", entity.Version, entity.UpdatedAt.UnixNano())`), compute the ETag directly from metadata to avoid hashing full payloads.
* **Set `ETag` Header on All Successful Mutations:** Always emit the updated `ETag` on `200 OK` or `201 Created` responses so clients have the new token for subsequent edits.

### Common Pitfalls
* **Ignoring Comma-Separated Values:** `If-Match` and `If-None-Match` can contain multiple comma-separated ETags or the wildcard `*`. Ensure your parser splits tags cleanly.
* **Weak vs. Strong ETags:** Prepending `W/` indicates a weak ETag (semantic equality), whereas non-prefixed tags indicate byte-for-byte strong equality.

---

## Code Walkthrough & Usage

### 1. Implementation (`etag_concurrency.go`)

```go
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

// CheckIfNoneMatch compares client's If-None-Match header with current ETag.
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

// CheckIfMatch compares client's If-Match header for optimistic updates.
func CheckIfMatch(r *http.Request, currentETag string) bool {
	clientETag := strings.TrimSpace(r.Header.Get("If-Match"))
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
```

### 2. Handler Read & Update Integration

```go
func HandleGetDocument(w http.ResponseWriter, r *http.Request) {
    doc := fetchDocument("doc-123")
    data, _ := json.Marshal(doc)
    etag := GenerateETag(data)

    if CheckIfNoneMatch(r, etag) {
        w.WriteHeader(http.StatusNotModified)
        return
    }

    w.Header().Set("ETag", etag)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(data)
}

func HandleUpdateDocument(w http.ResponseWriter, r *http.Request) {
    doc := fetchDocument("doc-123")
    data, _ := json.Marshal(doc)
    currentETag := GenerateETag(data)

    // Enforce optimistic concurrency
    if !CheckIfMatch(r, currentETag) {
        http.Error(w, "Conflict: resource modified by another transaction", http.StatusPreconditionFailed)
        return
    }

    // Apply updates...
}
```
