package errorx

import "errors"

type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindConflict
	KindBadRequest
	KindForbidden
	KindUnauthenticated
	KindServiceDegraded
)

func (k Kind) String() string {
	switch k {
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindBadRequest:
		return "bad_request"
	case KindForbidden:
		return "forbidden"
	case KindUnauthenticated:
		return "unauthorized"
	case KindServiceDegraded:
		return "service_degraded"
	default:
		return "unknown"
	}
}

// Status returns the canonical HTTP status code for the Kind.
// Single source of truth for HTTP mapping across the codebase.
// Use this in error handlers, middleware, and any caller that
// needs to translate an errorx.Error into an HTTP response.
func (k Kind) Status() int {
	switch k {
	case KindNotFound:
		return 404
	case KindConflict:
		return 409
	case KindBadRequest:
		return 400
	case KindForbidden:
		return 403
	case KindUnauthenticated:
		return 401
	case KindServiceDegraded:
		return 503
	default:
		return 500
	}
}

type Error interface {
	error
	Kind() Kind
	Unwrap() error
}

type errorx struct {
	kind    Kind
	message string
	cause   error
}

func (e *errorx) Error() string {
	return e.message
}

func (e *errorx) Kind() Kind {
	return e.kind
}

func (e *errorx) Unwrap() error {
	return e.cause
}

func New(kind Kind, message string) Error {
	return &errorx{
		kind:    kind,
		message: message,
	}
}

// Wrap wraps an existing error with a Kind. The original error is
// preserved via errors.Unwrap; if err is already an errorx.Error,
// its Kind takes precedence over the kind argument.
func Wrap(err error, kind Kind, message string) Error {
	if err == nil {
		return nil
	}
	var de Error
	if errors.As(err, &de) {
		kind = de.Kind()
	}
	wrapped := &errorx{
		kind:    kind,
		message: message,
	}
	wrapped.cause = err
	return wrapped
}
