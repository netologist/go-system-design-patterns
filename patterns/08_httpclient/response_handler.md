# Safe Response Body Handling and Status Code Validation

## Overview & Definition

When issuing outbound HTTP requests using Go's `http.Client`, interacting with `resp.Body` incorrectly is one of the most common sources of memory leaks and TCP connection pool starvation.

The **Safe Response Body Handling and Status Code Validation** pattern provides robust utilities that:
1. Guarantee `resp.Body.Close()` is always called.
2. Drain remaining unread body bytes into `io.Discard` via a bounded reader to ensure the underlying TCP socket is eligible for Keep-Alive connection reuse in `http.Transport`.
3. Protect against unbounded response payload sizes using `io.LimitReader(resp.Body, maxBytes+1)`.
4. Validate HTTP status codes with support for default 2xx ranges and explicit status code allowlists.

---

## Problem Statement

Naïve response handling in Go services leads to major production bugs:

* **Connection Pool Starvation (Leaked Sockets):** If `resp.Body.Close()` is forgotten, or if the response body is not fully read to EOF, Go cannot return the TCP connection to `http.Transport`'s idle pool. The connection is forcefully closed and discarded, resulting in TCP socket leaks and TIME_WAIT socket exhaustion.
* **Out-Of-Memory (OOM) via Unbounded Reads:** Calling `io.ReadAll(resp.Body)` without size bounds allows a compromised or misconfigured downstream service to stream gigabytes of data, causing immediate process crashes.
* **Silent Errors on Non-2xx Responses:** If code skips status code verification, an error page (e.g. HTML 502 Bad Gateway or 404 Not Found) is passed directly to `json.Unmarshal`, resulting in confusing deserialization syntax errors rather than actionable upstream error reporting.

---

## Architectural Mechanism & Flow

```
                      Outbound HTTP Call: resp, err := client.Do(req)
                                             |
                                             v
                      +---------------------------------------------+
                      |         ValidateStatusCode(resp)            |
                      +---------------------------------------------+
                                       /            \
                           [Unexpected Code]      [Valid 2xx / Allowed]
                                     /                \
                                    v                  v
                   +------------------------+  +-------------------------------+
                   | Return                 |  | SafeReadResponseBody(resp,    |
                   | ErrUnexpectedHTTPCode  |  |                    maxBytes)  |
                   +------------------------+  +-------------------------------+
                                                               |
                                                               v
                                               +-------------------------------+
                                               | defer func() {                |
                                               |   io.Copy(io.Discard, limit)  |
                                               |   resp.Body.Close()           |
                                               | }()                           |
                                               +-------------------------------+
                                                               |
                                                               v
                                               +-------------------------------+
                                               | io.LimitReader(body, max+1)   |
                                               | io.ReadAll(limitReader)       |
                                               +-------------------------------+
                                                               |
                                                  +------------+------------+
                                                  |                         |
                                                  v                         v
                                         [len(data) > maxBytes]   [len(data) <= maxBytes]
                                                  |                         |
                                                  v                         v
                                         +------------------+     +------------------+
                                         | Return Error:    |     | Return data      |
                                         | ResponseTooLarge |     | & Connection Reused|
                                         +------------------+     +------------------+
```

### Keep-Alive Draining Details
For `net/http` to reuse an HTTP/1.1 connection, the response body MUST be read to `io.EOF`. Draining up to 4KB (`io.LimitReader(resp.Body, 4096)`) into `io.Discard` before closing ensures fast keep-alive reuse without wasting CPU downloading gigabytes of unwanted remainder.

---

## Production Best Practices & Pitfalls

### Best Practices
* **Execute Cleanup in a Defer Statement:** Ensure the draining and closing logic is deferred immediately after verifying `resp != nil` and `err == nil`.
* **Cap Read Limits Appropriately:** Set explicit payload thresholds (e.g. 1 MB – 5 MB) based on API contracts.
* **Validate Status Codes Before Deserializing:** Always verify the status code before attempting JSON/Protobuf unmarshaling.

### Common Pitfalls
* **Ignoring the Error from `client.Do`:** Attempting to access `resp.Body` when `client.Do(req)` returned an error triggers a nil pointer dereference panic.
* **Calling `resp.Body.Close()` without Draining:** Closing an HTTP response body with unread data in the buffer causes Go's transport to send a TCP RST or terminate the connection rather than recycling it.

---

## Code Walkthrough & Usage

### 1. Implementation (`response_handler.go`)

```go
package httpclient

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrResponseTooLarge   = errors.New("response body exceeds max allowed size")
	ErrUnexpectedHTTPCode = errors.New("downstream returned unexpected HTTP status code")
)

// SafeReadResponseBody reads up to maxBytes from resp.Body, drains remainder to io.Discard for connection reuse, and closes body.
func SafeReadResponseBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("nil response or body")
	}

	// Always ensure body is closed and drained for HTTP Keep-Alive connection reuse
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
	}()

	if maxBytes <= 0 {
		maxBytes = 2 << 20 // 2 MB default
	}

	// Read up to maxBytes + 1 to detect truncation
	limitedReader := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading response body failed: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w (limit: %d bytes)", ErrResponseTooLarge, maxBytes)
	}

	return data, nil
}

// ValidateStatusCode checks that the response code is within the expected 2xx range or specific allowed codes.
func ValidateStatusCode(resp *http.Response, allowedCodes ...int) error {
	if resp == nil {
		return errors.New("nil response")
	}

	if len(allowedCodes) == 0 {
		// Default: accept 200-299
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("%w: got %d (%s)", ErrUnexpectedHTTPCode, resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		return nil
	}

	for _, code := range allowedCodes {
		if resp.StatusCode == code {
			return nil
		}
	}

	return fmt.Errorf("%w: got %d, expected %v", ErrUnexpectedHTTPCode, resp.StatusCode, allowedCodes)
}
```

### 2. Client Handler Integration

```go
func FetchInvoice(ctx context.Context, client *http.Client, invoiceID string) (*Invoice, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://billing.internal/invoices/"+invoiceID, nil)
    if err != nil {
        return nil, err
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }

    // Validate 200 OK
    if err := ValidateStatusCode(resp, http.StatusOK); err != nil {
        return nil, err
    }

    // Read bounded 512KB body safely and recycle socket
    data, err := SafeReadResponseBody(resp, 512*1024)
    if err != nil {
        return nil, err
    }

    var invoice Invoice
    if err := json.Unmarshal(data, &invoice); err != nil {
        return nil, fmt.Errorf("failed to parse invoice: %w", err)
    }

    return &invoice, nil
}
```
