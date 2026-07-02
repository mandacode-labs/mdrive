package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// FromError maps any error to (HTTP status, api.Error). The actual
// mapping logic lives in handler.FromError; this wrapper just lifts
// the result into the pkg/api type so callers in this package
// (recoverPanic, etc.) can write it directly.
func FromError(err error) (int, api.Error) {
	status, e := handler.FromError(err)
	return status, api.Error{
		Code:    api.ErrorCode(e.Code),
		Message: e.Message,
	}
}

const contentTypeJSON = "application/json"

func WriteError(w http.ResponseWriter, err error) {
	statusCode, apiErr := FromError(err)
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(apiErr); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func WriteJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}