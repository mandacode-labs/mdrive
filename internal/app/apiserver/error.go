package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"

	v1 "github.com/mandacode-labs/mdrive/api/rest/v1"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// FromError converts a domain error to (HTTP status code, v1.Error).
func FromError(err error) (int, v1.Error) {
	switch {
	// 404 Not Found
	case errors.Is(err, node.ErrNotFound),
		errors.Is(err, drive.ErrNotFound),
		errors.Is(err, user.ErrNotFound),
		errors.Is(err, node.ErrEntryNotFound),
		errors.Is(err, node.ErrNoContent):
		return http.StatusNotFound, v1.Error{Code: v1.CodeNotFound, Message: "not found"}

	// 409 Conflict
	case errors.Is(err, node.ErrEntryExists),
		errors.Is(err, node.ErrRevisionConflict):
		return http.StatusConflict, v1.Error{Code: v1.CodeConflict, Message: err.Error()}

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
		return http.StatusBadRequest, v1.Error{Code: v1.CodeBadRequest, Message: err.Error()}
	}

	return http.StatusInternalServerError, v1.Error{Code: v1.CodeInternal, Message: "internal error"}
}

// WriteError serializes the error as JSON and writes it to the response.
func WriteError(w http.ResponseWriter, err error) {
	statusCode, apiErr := FromError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(apiErr)
}

// WriteJSON writes a success response as JSON.
func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
