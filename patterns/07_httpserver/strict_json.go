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
