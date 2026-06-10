package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	domainerrors "github.com/mandacode-labs/mdrive/internal/errors"
	api "github.com/mandacode-labs/mdrive/pkg/api"
)

// NewError creates error response for ogen convenient errors.
func (h *Handler) NewError(ctx context.Context, err error) *api.ErrorStatusCode {
	var domainErr *domainerrors.Error
	if errors.As(err, &domainErr) {
		return &api.ErrorStatusCode{
			StatusCode: domainErr.StatusCode,
			Response: api.Error{
				Error: api.ErrorError{
					Type:    domainErr.Code,
					Message: domainErr.Message,
				},
			},
		}
	}

	// Handle ogen security errors
	var secErr *ogenerrors.SecurityError
	if errors.As(err, &secErr) {
		return &api.ErrorStatusCode{
			StatusCode: http.StatusUnauthorized,
			Response: api.Error{
				Error: api.ErrorError{
					Type:    "UNAUTHORIZED",
					Message: "authentication required",
				},
			},
		}
	}

	return &api.ErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response: api.Error{
			Error: api.ErrorError{
				Type:    "internal_error",
				Message: err.Error(),
			},
		},
	}
}

// ErrorHandler implements ogenerrors.ErrorHandler for proper HTTP status code mapping.
func (h *Handler) ErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	var domainErr *domainerrors.Error
	if errors.As(err, &domainErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(domainErr.StatusCode)
		resp := api.Error{
			Error: api.ErrorError{
				Type:    domainErr.Code,
				Message: domainErr.Message,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Handle ogen security errors
	var secErr *ogenerrors.SecurityError
	if errors.As(err, &secErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		resp := api.Error{
			Error: api.ErrorError{
				Type:    "UNAUTHORIZED",
				Message: "authentication required",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Default to 500 Internal Server Error
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	resp := api.Error{
		Error: api.ErrorError{
			Type:    "internal_error",
			Message: err.Error(),
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
