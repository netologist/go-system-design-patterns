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
