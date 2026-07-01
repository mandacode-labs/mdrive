// Package errorx is the single source of truth for typed errors.
//
// The Error interface exposes Kind() (semantic category) and Unwrap()
// (chain). Domain code constructs sentinels with New and grows chains
// with Wrap. Boundaries (HTTP handlers, CLI entrypoints, cron jobs)
// inspect errors via errors.As + Kind().Status() to map to wire codes,
// and rely on logx/Chain to surface the whole story in logs.
//
// Two rules:
//
//  1. Sentinels are domain catalog. Declare them once at package level
//     with New(Kind, "fixed: human-readable message"). Caller never
//     modifies the kind or message of a sentinel.
//
//  2. Wrap(err, "context: ...") appends a context message. The kind
//     is taken from the chain -- the first errorx.Error encountered
//     while unwrapping. Plain errors contribute no kind, so the chain
//     reads as KindUnknown for branches that never hit a sentinel.
package errorx

import (
	"errors"
	"fmt"
	"strings"
)

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

// New constructs a sentinel error with a fixed kind and message.
// Domain packages declare sentinels at package level using this;
// callers extend chains with Wrap but never modify a sentinel.
func New(kind Kind, message string) Error {
	return &errorx{
		kind:    kind,
		message: message,
	}
}

// Wrap appends a context message to an error chain. The kind is
// taken from the chain: the first errorx.Error encountered while
// unwrapping. Plain errors (no errorx.Error in the chain) produce
// KindUnknown, so the boundary can still ask for a status and get
// a sensible 500.
//
// Wrap never modifies the kind of a sentinel it wraps. The
// context-only message is what caller can and should add.
//
// Wrap(nil, ...) returns nil so callers can use it as a one-liner
// at every return site without nil-guards.
func Wrap(err error, message string, args ...any) Error {
	if err == nil {
		return nil
	}
	kind := KindUnknown
	var de Error
	if errors.As(err, &de) {
		kind = de.Kind()
	}
	return &errorx{
		kind:    kind,
		message: fmt.Sprintf(message, args...),
		cause:   err,
	}
}

// WrapSentinel attaches a sentinel to a plain error so the chain
// carries the sentinels kind and callers can errors.Is against it.
// Use this when a plain error (no errorx.Error in the chain) needs
// to be tagged with a domain-specific kind for downstream HTTP
// status mapping and log labels.
//
// The sentinel is recorded as the cause so errors.Is(err, sentinel)
// works on the returned chain. The outer message carries caller-
// supplied context for logs.
func WrapSentinel(sentinel Error, err error, message string, args ...any) Error {
	msg := fmt.Sprintf(message, args...)
	return &errorx{
		kind:    sentinel.Kind(),
		message: msg,
		cause:   sentinel,
	}
}

// KindOf returns the kind of the first errorx.Error in the chain.
// Returns KindUnknown if no errorx.Error is present.
func KindOf(err error) Kind {
	var de Error
	for err != nil {
		if errors.As(err, &de) {
			return de.Kind()
		}
		err = errors.Unwrap(err)
	}
	return KindUnknown
}

// Chain returns the error chain as "outer -> inner" so log
// readers can follow the full call path. Returns "" for nil
// and the single message for an error with no cause.
//
// Chain is what every logx.Error call should emit (or a logger
// that delegates to it). It's the single primitive that turns
// the propagation pattern into a useful diagnostic.
func Chain(err error) string {
	if err == nil {
		return ""
	}
	var parts []string
	for err != nil {
		parts = append(parts, err.Error())
		err = errors.Unwrap(err)
	}
	return strings.Join(parts, " -> ")
}
