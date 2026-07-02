package handler

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

// ErrorBody mirrors pkg/api.Error without importing it.
type ErrorBody struct {
	Code    Code
	Message string
}

// FromError returns the HTTP status and the wire body for err.
// errorx.Kind drives the status; non-errorx errors default to 500
// unless wrapped in *ogenerrors.SecurityError, which is treated
// as 401 (missing/unauthorized credentials).
func FromError(err error) (int, ErrorBody) {
	if status, code, msg, ok := mapErrorx(err); ok {
		return status, ErrorBody{Code: code, Message: msg}
	}
	if sec, ok := err.(*ogenerrors.SecurityError); ok && sec.Err != nil {
		if status, code, msg, ok2 := mapErrorx(sec.Err); ok2 {
			return status, ErrorBody{Code: code, Message: msg}
		}
	}
	return http.StatusInternalServerError, ErrorBody{
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
