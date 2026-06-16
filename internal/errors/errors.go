// Package errors provides structured domain error types for mdrive.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Error is a structured domain error with an HTTP-style status code and machine-readable code.
type Error struct {
	Code       string
	Message    string
	StatusCode int
	Details    map[string]any
	cause      error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *Error) Unwrap() error {
	return e.cause
}

// WithDetails returns a copy of the error with the given details merged in.
func (e *Error) WithDetails(details map[string]any) *Error {
	clone := *e
	clone.Details = make(map[string]any, len(e.Details)+len(details))
	for k, v := range e.Details {
		clone.Details[k] = v
	}
	for k, v := range details {
		clone.Details[k] = v
	}
	return &clone
}

// WithCause returns a copy of the error wrapping the given cause.
func (e *Error) WithCause(cause error) *Error {
	clone := *e
	clone.cause = cause
	return &clone
}

// Constructors for common error types.

func BadRequest(msg string) *Error {
	return &Error{Code: "bad_request", Message: msg, StatusCode: http.StatusBadRequest}
}

func Unauthorized(msg string) *Error {
	return &Error{Code: "unauthorized", Message: msg, StatusCode: http.StatusUnauthorized}
}

func Forbidden(msg string) *Error {
	return &Error{Code: "forbidden", Message: msg, StatusCode: http.StatusForbidden}
}

func NotFound(msg string) *Error {
	return &Error{Code: "not_found", Message: msg, StatusCode: http.StatusNotFound}
}

func Conflict(msg string) *Error {
	return &Error{Code: "conflict", Message: msg, StatusCode: http.StatusConflict}
}

func Internal(msg string) *Error {
	return &Error{Code: "internal", Message: msg, StatusCode: http.StatusInternalServerError}
}

func ServiceUnavailable(msg string) *Error {
	return &Error{Code: "service_unavailable", Message: msg, StatusCode: http.StatusServiceUnavailable}
}

// Wrap functions for wrapping a standard error with a domain error type.

func WrapBadRequest(err error, msg string) *Error {
	return BadRequest(msg).WithCause(err)
}

func WrapUnauthorized(err error, msg string) *Error {
	return Unauthorized(msg).WithCause(err)
}

func WrapForbidden(err error, msg string) *Error {
	return Forbidden(msg).WithCause(err)
}

func WrapNotFound(err error, msg string) *Error {
	return NotFound(msg).WithCause(err)
}

func WrapConflict(err error, msg string) *Error {
	return Conflict(msg).WithCause(err)
}

func WrapInternal(err error, msg string) *Error {
	return Internal(msg).WithCause(err)
}

// As is a convenience wrapper around errors.As for *Error targets.
func As(err error, target any) bool {
	return errors.As(err, target)
}
