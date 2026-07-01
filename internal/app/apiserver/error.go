package apiserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// FromError converts a domain error to (HTTP status code, api.Error).
// Delegates the status mapping to errorx.Kind.Status() so the same
// Kind produces the same status everywhere in the codebase.
func FromError(err error) (int, api.Error) {
	var de errorx.Error
	if errors.As(err, &de) {
		switch de.Kind() {
		case errorx.KindNotFound:
			return de.Kind().Status(), api.Error{Code: api.ErrorCodeNotFound, Message: "not found"}
		case errorx.KindConflict:
			return de.Kind().Status(), api.Error{Code: api.ErrorCodeConflict, Message: err.Error()}
		case errorx.KindBadRequest:
			return de.Kind().Status(), api.Error{Code: api.ErrorCodeBadRequest, Message: err.Error()}
		case errorx.KindForbidden:
			return de.Kind().Status(), api.Error{Code: api.ErrorCodeForbidden, Message: "permission denied"}
		case errorx.KindUnauthenticated:
			return de.Kind().Status(), api.Error{Code: api.ErrorCodeUnauthorized, Message: "unauthenticated"}
		case errorx.KindServiceDegraded:
			return de.Kind().Status(), api.Error{Code: api.ErrorCodeInternal, Message: err.Error()}
		}
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