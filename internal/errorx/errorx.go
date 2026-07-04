// Package errorx is the single source of truth for typed errors.
//
// Kinds are gRPC-aligned (https://grpc.io/docs/guides/status-codes/).
// Each kind maps to an HTTP status via Kind.Status(). The wire-level
// "code" (machine-readable short string) is mapped from Kind in the
// API middleware, NOT in this package — errorx stays HTTP / API
// unaware so the same errors can be reused for non-HTTP transports.
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

// Kind is the gRPC-aligned error category. Numbering mirrors the
// canonical gRPC status codes for the well-known values, with
// domain-specific kinds added at the end of the well-known range
// to keep status code stability for existing callers.
type Kind int

const (
	// KindUnknown is the zero value; treat it as 500.
	KindUnknown Kind = iota
	// KindInternal: 500. Server-side bug, panic, or invariant
	// violation. Not retryable without code change.
	KindInternal
	// KindCanceled: 499 (or 408 in HTTP/1.1). Client cancelled
	// the request or the deadline elapsed at the caller.
	KindCanceled
	// KindInvalidArgument: 400. Caller passed a malformed or
	// semantically wrong argument (e.g. empty required field,
	// path that resolves above the drive root).
	KindInvalidArgument
	// KindDeadlineExceeded: 504. Operation took longer than the
	// caller's deadline. Retryable.
	KindDeadlineExceeded
	// KindNotFound: 404. Entity does not exist or is not
	// visible to the caller.
	KindNotFound
	// KindAlreadyExists: 409. Uniqueness conflict on create
	// (e.g. duplicate name when unique).
	KindAlreadyExists
	// KindPermissionDenied: 403. Caller is authenticated but
	// not authorized for the resource.
	KindPermissionDenied
	// KindResourceExhausted: 429. Quota, rate limit, or
	// storage cap hit.
	KindResourceExhausted
	// KindFailedPrecondition: 400 / 412. State of the system
	// blocks the operation (e.g. trying to delete a non-empty
	// directory without recursive=true).
	KindFailedPrecondition
	// KindAborted: 409. Optimistic-concurrency or transaction
	// conflict; retryable.
	KindAborted
	// KindOutOfRange: 400. Valid input that the system cannot
	// process (e.g. file too large for the configured cap).
	KindOutOfRange
	// KindUnimplemented: 501. Operation is not yet wired up
	// for this resource class.
	KindUnimplemented
	// KindUnavailable: 503. Upstream or dependency is down;
	// retryable. Prefer over KindInternal for transport-level
	// failures.
	KindUnavailable
	// KindDataLoss: 500. Unrecoverable corruption. Higher
	// severity than KindInternal.
	KindDataLoss
	// KindUnauthenticated: 401. No valid session or credential.
	KindUnauthenticated

	// --- domain-specific kinds (mdrive only) ---

	// KindRevisionConflict: 409. Concurrent write to the same
	// node revision; client must re-fetch and retry.
	KindRevisionConflict
)

// String returns the gRPC-style short name. Used in logs and the
// API middleware message field. NOT the wire code — the API
// middleware owns that mapping.
// String returns the gRPC-style short name. Used in logs and the
// API middleware message field. NOT the wire code — the API
// middleware owns that mapping.
func (k Kind) String() string {
	switch k {
	case KindCanceled:
		return "canceled"
	case KindInvalidArgument:
		return "invalid_argument"
	case KindDeadlineExceeded:
		return "deadline_exceeded"
	case KindNotFound:
		return "not_found"
	case KindAlreadyExists:
		return "already_exists"
	case KindPermissionDenied:
		return "permission_denied"
	case KindResourceExhausted:
		return "resource_exhausted"
	case KindFailedPrecondition:
		return "failed_precondition"
	case KindAborted:
		return "aborted"
	case KindOutOfRange:
		return "out_of_range"
	case KindUnimplemented:
		return "unimplemented"
	case KindInternal:
		return "internal"
	case KindUnavailable:
		return "unavailable"
	case KindDataLoss:
		return "data_loss"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindRevisionConflict:
		return "revision_conflict"
	default:
		return "unknown"
	}
}

// Status returns the HTTP status code for the kind. Mirrors
// https://cloud.google.com/apis/design/errors#handling_errors
// where applicable.
func (k Kind) Status() int {
	switch k {
	case KindCanceled:
		return 499
	case KindInvalidArgument:
		return 400
	case KindDeadlineExceeded:
		return 504
	case KindNotFound:
		return 404
	case KindAlreadyExists:
		return 409
	case KindPermissionDenied:
		return 403
	case KindResourceExhausted:
		return 429
	case KindFailedPrecondition:
		return 412
	case KindAborted:
		return 409
	case KindOutOfRange:
		return 400
	case KindUnimplemented:
		return 501
	case KindInternal:
		return 500
	case KindUnavailable:
		return 503
	case KindDataLoss:
		return 500
	case KindUnauthenticated:
		return 401
	case KindRevisionConflict:
		return 409
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
	if de, ok := errors.AsType[Error](err); ok {
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
// this instead of errors.Is: sentinels are inlined at the point
// of construction rather than declared as package-level
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

// IsKind reports whether err's chain contains an errorx.Error
// with the given kind. Use in tests and conditional branches
// instead of KindOf(err) == k, which is a one-liner the optimizer
// can't help with when a chain is involved.
func IsKind(err error, k Kind) bool {
	return KindOf(err) == k
}
