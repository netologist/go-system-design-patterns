package errorspattern

import (
	"errors"
	"net/http"
)

// PublicHTTPError represents sanitized error payload safe for client consumption.
type PublicHTTPError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
}

// ErrorMapper coordinates centralized mapping from Go errors to HTTP status and public payload.
type ErrorMapper struct{}

func NewErrorMapper() *ErrorMapper {
	return &ErrorMapper{}
}

// MapToHTTP converts any internal error into an HTTP status code and safe public error representation.
func (m *ErrorMapper) MapToHTTP(err error) (int, PublicHTTPError) {
	if err == nil {
		return http.StatusOK, PublicHTTPError{
			StatusCode: http.StatusOK,
			Code:       "OK",
			Message:    "Success",
		}
	}

	// 1. Check if it is a typed DomainError
	if domainErr, ok := ExtractDomainError(err); ok {
		switch domainErr.Kind {
		case KindNotFound:
			return http.StatusNotFound, PublicHTTPError{
				StatusCode: http.StatusNotFound,
				Code:       "RESOURCE_NOT_FOUND",
				Message:    domainErr.Message,
			}
		case KindConflict:
			return http.StatusConflict, PublicHTTPError{
				StatusCode: http.StatusConflict,
				Code:       "RESOURCE_CONFLICT",
				Message:    domainErr.Message,
			}
		case KindValidation:
			return http.StatusUnprocessableEntity, PublicHTTPError{
				StatusCode: http.StatusUnprocessableEntity,
				Code:       "VALIDATION_ERROR",
				Message:    domainErr.Message,
				Field:      domainErr.Field,
			}
		case KindUnauthorized:
			return http.StatusUnauthorized, PublicHTTPError{
				StatusCode: http.StatusUnauthorized,
				Code:       "UNAUTHORIZED",
				Message:    "Authentication required",
			}
		case KindForbidden:
			return http.StatusForbidden, PublicHTTPError{
				StatusCode: http.StatusForbidden,
				Code:       "FORBIDDEN",
				Message:    "You do not have permission to perform this action",
			}
		case KindInternal:
			return http.StatusInternalServerError, PublicHTTPError{
				StatusCode: http.StatusInternalServerError,
				Code:       "INTERNAL_ERROR",
				Message:    "An unexpected error occurred. Please try again later.",
			}
		}
	}

	// 2. Check standard sentinel errors
	if errors.Is(err, ErrNotFound) {
		return http.StatusNotFound, PublicHTTPError{
			StatusCode: http.StatusNotFound,
			Code:       "NOT_FOUND",
			Message:    "Resource not found",
		}
	}
	if errors.Is(err, ErrConflict) {
		return http.StatusConflict, PublicHTTPError{
			StatusCode: http.StatusConflict,
			Code:       "CONFLICT",
			Message:    "Resource conflict",
		}
	}

	// 3. Fallback: Any unknown or DB/internal error MUST NEVER leak internal details to client
	return http.StatusInternalServerError, PublicHTTPError{
		StatusCode: http.StatusInternalServerError,
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    "An internal error occurred. Please contact support.",
	}
}
