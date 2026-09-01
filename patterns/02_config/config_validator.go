package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ValidationError represents an error on a specific config field.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("field '%s': %s", e.Field, e.Message)
}

// ConfigValidator provides composable validation rules for application configuration.
type ConfigValidator struct {
	errors []ValidationError
}

// NewConfigValidator creates a new validator.
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		errors: make([]ValidationError, 0),
	}
}

// RequireNotEmpty checks that a string field is not empty.
func (v *ConfigValidator) RequireNotEmpty(field, value string) {
	if strings.TrimSpace(value) == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "cannot be empty",
		})
	}
}

// ValidatePort checks that the port is within standard valid TCP ranges.
func (v *ConfigValidator) ValidatePort(field string, port int) {
	if port < 1 || port > 65535 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("port %d is out of range [1-65535]", port),
		})
	}
}

// ValidatePositiveDuration checks that a duration is > 0.
func (v *ConfigValidator) ValidatePositiveDuration(field string, d time.Duration) {
	if d <= 0 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "duration must be strictly positive",
		})
	}
}

// ValidateURL checks that a URL is well-formed and has one of the allowed schemes.
func (v *ConfigValidator) ValidateURL(field, rawURL string, allowedSchemes ...string) {
	if strings.TrimSpace(rawURL) == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "URL cannot be empty",
		})
		return
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("invalid URL format: %s", rawURL),
		})
		return
	}

	if len(allowedSchemes) > 0 {
		matched := false
		for _, s := range allowedSchemes {
			if strings.EqualFold(u.Scheme, s) {
				matched = true
				break
			}
		}
		if !matched {
			v.errors = append(v.errors, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("scheme '%s' is not allowed (expected %v)", u.Scheme, allowedSchemes),
			})
		}
	}
}

// ValidateEnum checks that value is in allowed options.
func (v *ConfigValidator) ValidateEnum(field, value string, allowed ...string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: fmt.Sprintf("value '%s' not in allowed choices %v", value, allowed),
	})
}

// Result returns an aggregated error if any validation failed.
func (v *ConfigValidator) Result() error {
	if len(v.errors) == 0 {
		return nil
	}
	var errs []error
	for _, e := range v.errors {
		errs = append(errs, e)
	}
	return fmt.Errorf("configuration validation errors: %w", errors.Join(errs...))
}
