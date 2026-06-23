package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// FromError converts a domain error to (HTTP status code, api.Error).
func FromError(err error) (int, api.Error) {
	switch {
	case errors.Is(err, ogenerrors.ErrSecurityRequirementIsNotSatisfied):
		return http.StatusUnauthorized, api.Error{Code: api.ErrorCodeUnauthorized, Message: "unauthorized"}

	case errors.Is(err, permission.ErrPermission):
		return http.StatusForbidden, api.Error{Code: api.ErrorCodeForbidden, Message: "permission denied"}

	case errors.Is(err, node.ErrNotFound),
		errors.Is(err, drive.ErrNotFound),
		errors.Is(err, user.ErrNotFound),
		errors.Is(err, node.ErrEntryNotFound),
		errors.Is(err, node.ErrNoContent),
		errors.Is(err, vfs.ErrNotFound),
		errors.Is(err, upload.ErrObjectNotUploaded):
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
		errors.Is(err, user.ErrNameRequired),
		errors.Is(err, vfs.ErrInvalidPath),
		errors.Is(err, vfs.ErrCrossDrive),
		errors.Is(err, upload.ErrUploadMismatch):
		return http.StatusBadRequest, api.Error{Code: api.ErrorCodeBadRequest, Message: err.Error()}
	}

	var secErr *ogenerrors.SecurityError
	if errors.As(err, &secErr) {
		return http.StatusUnauthorized, api.Error{Code: api.ErrorCodeUnauthorized, Message: "unauthorized"}
	}

	return http.StatusInternalServerError, api.Error{Code: api.ErrorCodeInternal, Message: "internal error"}
}

const contentTypeJSON = "application/json"

// WriteError writes an error response to w.
func WriteError(w http.ResponseWriter, err error) {
	statusCode, apiErr := FromError(err)
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(apiErr); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// WriteJSON writes a JSON response to w.
func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
