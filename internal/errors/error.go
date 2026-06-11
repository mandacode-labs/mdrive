package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Error represents a structured API error.
type Error struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	StatusCode int            `json:"-"`
	Details    map[string]any `json:"details,omitempty"`
	cause      error          // wrapped error
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error.
func (e *Error) Unwrap() error {
	return e.cause
}

// New creates a new Error.
func New(code, message string, statusCode int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// WithDetails adds details to the error.
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// Common error constructors
func BadRequest(message string) *Error {
	return New("BAD_REQUEST", message, http.StatusBadRequest)
}

func Unauthorized(message string) *Error {
	return New("UNAUTHORIZED", message, http.StatusUnauthorized)
}

func Forbidden(message string) *Error {
	return New("FORBIDDEN", message, http.StatusForbidden)
}

func NotFound(message string) *Error {
	return New("NOT_FOUND", message, http.StatusNotFound)
}

func Conflict(message string) *Error {
	return New("CONFLICT", message, http.StatusConflict)
}

func Internal(message string) *Error {
	return New("INTERNAL_ERROR", message, http.StatusInternalServerError)
}

func ServiceUnavailable(message string) *Error {
	return New("SERVICE_UNAVAILABLE", message, http.StatusServiceUnavailable)
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusNotFound
	}
	return false
}

// IsBadRequest checks if the error is a bad request error.
func IsBadRequest(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusBadRequest
	}
	return false
}

// IsAlreadyExists checks if the error is a conflict error (already exists).
func IsAlreadyExists(err error) bool {
	return IsConflict(err)
}

// IsConflict checks if the error is a conflict error.
func IsConflict(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusConflict
	}
	return false
}

// IsUnauthorized checks if the error is an unauthorized error.
func IsUnauthorized(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsForbidden checks if the error is a forbidden error.
func IsForbidden(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode == http.StatusForbidden
	}
	return false
}

// FromError converts a standard error to an Error.
func FromError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return WrapInternal(err, err.Error())
}

// Wrap wraps a standard error with a new Error code and message.
func Wrap(err error, code, message string, statusCode int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		cause:      err,
	}
}

// WrapNotFound wraps an error as a not found error.
func WrapNotFound(err error, message string) *Error {
	return Wrap(err, "NOT_FOUND", message, http.StatusNotFound)
}

// WrapBadRequest wraps an error as a bad request error.
func WrapBadRequest(err error, message string) *Error {
	return Wrap(err, "BAD_REQUEST", message, http.StatusBadRequest)
}

// WrapInternal wraps an error as an internal error.
func WrapInternal(err error, message string) *Error {
	return Wrap(err, "INTERNAL_ERROR", message, http.StatusInternalServerError)
}
