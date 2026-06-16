// Package v1 defines the API service contract: shared error types, constants,
// and schema models used by both the OpenAPI spec and the generated ogen code.
package v1

// ErrorCode is a machine-readable error identifier stable across API versions.
type ErrorCode string

const (
	CodeNotFound     ErrorCode = "not_found"
	CodeConflict     ErrorCode = "conflict"
	CodeBadRequest   ErrorCode = "bad_request"
	CodeForbidden    ErrorCode = "forbidden"
	CodeUnauthorized ErrorCode = "unauthorized"
	CodeInternal     ErrorCode = "internal"
	CodeRevisionConflict ErrorCode = "revision_conflict"
)

// Error is the standard JSON error response body.
type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}
