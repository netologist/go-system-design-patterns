# Strict JSON Decoding

## Overview & Definition

In production Go backend APIs, standard `json.Unmarshal` and default `json.NewDecoder` behavior silently ignores unknown fields and ignores trailing junk/multiple concatenated JSON objects. The **Strict JSON Decoding** pattern implements a hardened decoding pipeline that:
1. **Rejects unknown fields** using `decoder.DisallowUnknownFields()`, catching typos, schema mismatches, and extraneous attributes.
2. **Enforces single-document boundaries**, rejecting payloads with trailing garbage or multiple concatenated JSON documents.
3. **Classifies error conditions**, returning actionable, typed errors (`json.SyntaxError`, `json.UnmarshalTypeError`, empty body detection, etc.).

This pattern guarantees strict schema adherence, predictable request payloads, and prevents subtle security and validation bugs caused by payload smuggling or client parameter pollution.

---

## Problem Statement

Standard Go JSON deserialization defaults introduce subtle bugs and security vulnerabilities:

* **Silent Field Discarding & Typos:** If a client sends `{"emali": "user@example.com"}` instead of `{"email": "..."}`, standard deserialization silently ignores `emali`, leaving the Go struct field initialized to its zero value (`""`). The request might proceed with invalid data or default states.
* **JSON Injection & Smuggling via Multiple Documents:** A default `json.NewDecoder(r.Body).Decode(&dst)` reads only the first valid JSON token/object from the stream. If an attacker sends `{"action":"view"}{"action":"admin_delete"}`, the server decodes the first object while trailing data remains in the stream, potentially exploiting downstream pipeline stages or proxies.
* **Poor Developer Experience:** Standard `json.Unmarshal` returns opaque error strings that do not clearly specify offset locations or offending field names for client debugging.

---

## Architectural Mechanism & Flow

```
                      +------------------------------------------+
                      |         Incoming io.Reader Stream        |
                      +------------------------------------------+
                                           |
                                           v
                      +------------------------------------------+
                      |         json.NewDecoder(stream)          |
                      |     decoder.DisallowUnknownFields()      |
                      +------------------------------------------+
                                           |
                                           v
                              +--------------------------+
                              |   decoder.Decode(dst)    |
                              +--------------------------+
                                     /            \
                          [Decode Error]        [Success]
                                   /                \
                                  v                  v
                   +---------------------------+  +-------------------------------+
                   | Classify Error Type:      |  | Second Decode Call:           |
                   | - SyntaxError (offset)    |  | decoder.Decode(&extra)        |
                   | - UnmarshalTypeError      |  +-------------------------------+
                   | - Unknown Field error     |             /             \
                   | - EOF (Empty body)        |     [Is io.EOF]     [Not io.EOF]
                   +---------------------------+          /                   \
                                                         v                     v
                                                 +---------------+   +--------------------+
                                                 | Return Valid  |   | Return Error:      |
                                                 | Struct Result |   | ErrMultipleJSONVals|
                                                 +---------------+   +--------------------+
```

### Key Mechanism Details
1. `decoder.DisallowUnknownFields()` causes `Decode` to return an error when the input contains keys that do not match exported fields in the target struct.
2. The error string is inspected for prefix `"json: unknown field"` to return a sentinel `ErrUnknownField`.
3. After the primary decode, a secondary probe `decoder.Decode(&extra)` is executed. If the secondary read returns anything other than `io.EOF`, the payload contains extra trailing JSON tokens or multiple objects, which is rejected with `ErrMultipleJSONVals`.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Combine with Body Limiting:** Always pair `DecodeStrictJSON` with `http.MaxBytesReader` so decoding does not consume unbounded memory from an attacker-controlled stream.
* **Return Granular HTTP 400 Responses:** Convert `json.SyntaxError` and `json.UnmarshalTypeError` into clear client-facing problem details (RFC 7807/9457) pinpointing the field name and error reason.
* **Use Pointers or Nullable Types for Optional Fields:** Since unknown fields are rejected, explicitly model optional attributes using pointers (`*string`), `nil`-able slices, or dedicated optional wrapper types.

### Common Pitfalls
* **Breaking Non-Breaking Additions in Public APIs:** If external third parties or webhooks send varying or evolving payloads, strict unknown field rejection will break when the webhook provider adds new metadata fields. Apply strict decoding primarily to internal APIs and client-facing mutation endpoints.
* **Buffer Reuse:** Avoid reading the entire stream into a `[]byte` with `io.ReadAll` before passing to `DecodeStrictJSON` unless re-reading is required, as this doubles heap allocation.

---

## Code Walkthrough & Usage

### 1. Implementation (`strict_json.go`)

```go
package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrUnknownField     = errors.New("json contains unknown field")
	ErrMultipleJSONVals = errors.New("request body must contain only a single JSON object")
	ErrEmptyBody        = errors.New("request body cannot be empty")
)

// DecodeStrictJSON decodes a JSON stream, rejecting unknown fields and ensuring only one JSON document is present.
func DecodeStrictJSON(r io.Reader, dst any) error {
	if r == nil {
		return ErrEmptyBody
	}

	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalTypeErr *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxErr):
			return fmt.Errorf("malformed JSON at byte offset %d: %w", syntaxErr.Offset, err)
		case errors.As(err, &unmarshalTypeErr):
			return fmt.Errorf("invalid value for field '%s': %w", unmarshalTypeErr.Field, err)
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			return fmt.Errorf("%w: %s", ErrUnknownField, err.Error())
		case errors.Is(err, io.EOF):
			return ErrEmptyBody
		default:
			return fmt.Errorf("json decode error: %w", err)
		}
	}

	// Ensure there are no extra JSON objects or trailing bytes
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrMultipleJSONVals
	}

	return nil
}
```

### 2. Handler Usage Example

```go
type CreateAccountPayload struct {
    Username string `json:"username"`
    Email    string `json:"email"`
}

func HandleCreateAccount(w http.ResponseWriter, r *http.Request) {
    var payload CreateAccountPayload
    if err := DecodeStrictJSON(r.Body, &payload); err != nil {
        switch {
        case errors.Is(err, ErrUnknownField):
            http.Error(w, err.Error(), http.StatusBadRequest)
        case errors.Is(err, ErrMultipleJSONVals):
            http.Error(w, "Multiple JSON documents not allowed", http.StatusBadRequest)
        case errors.Is(err, ErrEmptyBody):
            http.Error(w, "Request body cannot be empty", http.StatusBadRequest)
        default:
            http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
        }
        return
    }

    // Process valid payload
    w.WriteHeader(http.StatusCreated)
}
```
