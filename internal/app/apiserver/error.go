package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// APIError is the JSON-serializable error response.
type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// FromError converts a domain error to (HTTP status code, APIError).
//
// Each domain package defines its own sentinel errors. This function is the
// single place that maps them to HTTP semantics. New domains should be
// added here as their errors are introduced.
func FromError(err error) (int, *APIError) {
	switch {
	// 404 Not Found
	case errors.Is(err, node.ErrNotFound),
		errors.Is(err, drive.ErrNotFound),
		errors.Is(err, user.ErrNotFound),
		errors.Is(err, node.ErrEntryNotFound),
		errors.Is(err, node.ErrNoContent):
		return http.StatusNotFound, &APIError{
			Code:    "not_found",
			Message: "resource not found",
		}

	// 409 Conflict
	case errors.Is(err, node.ErrEntryExists):
		return http.StatusConflict, &APIError{
			Code:    "entry_exists",
			Message: "entry already exists",
		}

	// 400 Bad Request
	case errors.Is(err, node.ErrInvalidName),
		errors.Is(err, node.ErrInvalidType),
		errors.Is(err, node.ErrInvalidReference),
		errors.Is(err, node.ErrInvalidSize),
		errors.Is(err, node.ErrNotDirectory),
		errors.Is(err, node.ErrContentTooLarge),
		errors.Is(err, drive.ErrInvalidName),
		errors.Is(err, drive.ErrInvalidBucket),
		errors.Is(err, drive.ErrInvalidRegion),
		errors.Is(err, drive.ErrInvalidCredentials),
		errors.Is(err, user.ErrProviderRequired),
		errors.Is(err, user.ErrProviderIDRequired),
		errors.Is(err, user.ErrNameRequired):
		return http.StatusBadRequest, &APIError{
			Code:    "bad_request",
			Message: "invalid request",
		}
	}

	// Default: 500 Internal Server Error
	return http.StatusInternalServerError, &APIError{
		Code:    "internal",
		Message: "internal server error",
	}
}

// WriteError writes the error as an HTTP response.
func WriteError(w http.ResponseWriter, err error) {
	statusCode, apiErr := FromError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(apiErr)
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
