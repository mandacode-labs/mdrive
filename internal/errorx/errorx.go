// Package errorx is the single source of truth for typed errors.
//
//  1. New(Kind, "fixed: human-readable message") returns an
//     inline sentinel with that kind and message.
//
//  2. Wrap(err, "context: ...", kinds...) appends a context
//     message to an error chain. Kind is taken from the chain
//     (the first errorx.Error found) unless a Kind override is
//     passed. Wrap(nil, ...) returns nil so callers can use it
//     as a one-liner at every return site.
//
//  3. KindOf(err) traverses the chain to the first errorx.Error
//     and returns its Kind (or KindUnknown). Use this instead of
//     errors.Is: sentinels are inlined at call sites, so chain
//     identity is by Kind, not by sentinel object.
//
// Error() renders the chain as "outer: inner" (Go's fmt %w
// semantics) so the same call site feeds both user-facing
// response bodies and server logs.
package errorx

import (
	"errors"
	"fmt"
)

type Kind int

const (
	KindUnknown Kind = iota
	KindInternal
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
	case KindInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// Status returns the HTTP status code corresponding to the error kind.
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

// Error renders "outer: inner" so a single call covers both
// HTTP response bodies and log output. The cause is joined with
// ": " following Go's fmt %w convention.
func (e *errorx) Error() string {
	if e.cause == nil {
		return e.message
	}
	return fmt.Sprintf("%s: %v", e.message, e.cause)
}

func (e *errorx) Kind() Kind {
	return e.kind
}

func (e *errorx) Unwrap() error {
	return e.cause
}

// New constructs an error with a fixed kind and message. Use at
// the point where an error originates (call sites), not as a
// package-level catalog; for chains use Wrap.
func New(kind Kind, message string) Error {
	return &errorx{
		kind:    kind,
		message: message,
	}
}

// Wrap appends a context message to an error chain. Kind is taken
// from the chain (the first errorx.Error found by errors.As);
// pass Kind arguments to override. Wrap(nil, ...) returns nil
// so callers can use it as a one-liner at every return site.
func Wrap(err error, message string, kind ...Kind) Error {
	if err == nil {
		return nil
	}
	k := KindUnknown
	var de Error
	if errors.As(err, &de) {
		k = de.Kind()
	}
	if len(kind) > 0 {
		k = kind[0]
	}
	return &errorx{
		kind:    k,
		message: message,
		cause:   err,
	}
}

// KindOf returns the kind of the first errorx.Error in the chain.
// Returns KindUnknown if no errorx.Error is present. Callers use
// this instead of errors.Is because sentinels are inlined at the
// point of construction rather than declared as package-level
// variables, so chain identity is by Kind, not by object.
func KindOf(err error) Kind {
	for err != nil {
		if de, ok := errors.AsType[Error](err); ok {
			return de.Kind()
		}
		err = errors.Unwrap(err)
	}
	return KindUnknown
}
