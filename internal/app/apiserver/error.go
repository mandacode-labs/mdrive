package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mandacode-labs/mdrive/pkg/api"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// FromError converts a domain error to (HTTP status code, api.Error).
func FromError(err error) (int, api.Error) {
	switch {
	case errors.Is(err, node.ErrNotFound),
		errors.Is(err, drive.ErrNotFound),
		errors.Is(err, user.ErrNotFound),
		errors.Is(err, node.ErrEntryNotFound),
		errors.Is(err, node.ErrNoContent):
		return http.StatusNotFound, api.Error{Code: api.ErrorCodeNotFound, Message: "not found"}

	case errors.Is(err, node.ErrEntryExists),
		errors.Is(err, node.ErrRevisionConflict):
		return http.StatusConflict, api.Error{Code: api.ErrorCodeConflict, Message: err.Error()}

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
		return http.StatusBadRequest, api.Error{Code: api.ErrorCodeBadRequest, Message: err.Error()}
	}

	return http.StatusInternalServerError, api.Error{Code: api.ErrorCodeInternal, Message: "internal error"}
}

func WriteError(w http.ResponseWriter, err error) {
	statusCode, apiErr := FromError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(apiErr)
}

func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
