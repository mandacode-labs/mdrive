// Package apierr provides the canonical mapping from any Go error
// to an HTTP status code plus the wire-format api.Error body.
//
// errorx.Kind is the single source of truth for the status code
// (see internal/errorx.Kind.Status). This package exists only to
// peel off ogen's *ogenerrors.SecurityError wrapper, which has no
// Unwrap method, so errors.As cannot reach the inner errorx.Error
// directly. Without that peel, missing-session errors from
// SecurityHandler surface as 503 instead of 401.
package apierr

import (
	"errors"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Code is the package-internal api.ErrorCode enum alias.
// Importing pkg/api from here would create a cycle (apiserver ->
// handler -> apiserver -> pkg/api). The string values are stable
// across versions and match the OpenAPI spec.
type Code string

const (
	codeNotFound        Code = "not_found"
	codeConflict        Code = "conflict"
	codeBadRequest      Code = "bad_request"
	codeForbidden       Code = "forbidden"
	codeUnauthorized    Code = "unauthorized"
	codeServiceDegraded Code = "service_degraded"
	codeUnknown         Code = "unknown"
	codeInternal        Code = "internal_error"
)

// Error mirrors pkg/api.Error without importing it.
type Error struct {
	Code    Code
	Message string
}

// FromError returns the HTTP status and the wire body for err.
// errorx.Kind drives the status; non-errorx errors default to 500
// unless wrapped in *ogenerrors.SecurityError, which is treated
// as 401 (missing/unauthorized credentials).
func FromError(err error) (int, Error) {
	if status, code, msg, ok := mapErrorx(err); ok {
		return status, Error{Code: code, Message: msg}
	}
	if sec, ok := err.(*ogenerrors.SecurityError); ok && sec.Err != nil {
		if status, code, msg, ok2 := mapErrorx(sec.Err); ok2 {
			return status, Error{Code: code, Message: msg}
		}
	}
	return http.StatusInternalServerError, Error{
		Code:    codeInternal,
		Message: "internal error",
	}
}

func mapErrorx(err error) (int, Code, string, bool) {
	var de errorx.Error
	if !errors.As(err, &de) {
		return 0, "", "", false
	}
	return de.Kind().Status(), apiCodeForKind(de.Kind()), de.Error(), true
}

func apiCodeForKind(k errorx.Kind) Code {
	switch k {
	case errorx.KindNotFound:
		return codeNotFound
	case errorx.KindConflict:
		return codeConflict
	case errorx.KindBadRequest:
		return codeBadRequest
	case errorx.KindForbidden:
		return codeForbidden
	case errorx.KindUnauthenticated:
		return codeUnauthorized
	case errorx.KindServiceDegraded:
		return codeServiceDegraded
	case errorx.KindUnknown:
		return codeUnknown
	default:
		return codeInternal
	}
}