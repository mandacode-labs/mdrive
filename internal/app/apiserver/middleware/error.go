package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/ogen-go/ogen/middleware"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// kindToCode is the single mapping from errorx.Kind to the
// machine-readable api.ErrorCode wire value. errorx stays
// API-agnostic; this table lives in the API middleware so
// the same Kind can be reused over non-HTTP transports.
//
// Mirrors the gRPC code set https://grpc.io/docs/guides/status-codes/
// with one mdrive-specific extension (revision_conflict).
var kindToCode = map[errorx.Kind]api.ErrorCode{
	errorx.KindCanceled:           api.ErrorCodeCanceled,
	errorx.KindInvalidArgument:    api.ErrorCodeInvalidArgument,
	errorx.KindDeadlineExceeded:   api.ErrorCodeDeadlineExceeded,
	errorx.KindNotFound:           api.ErrorCodeNotFound,
	errorx.KindAlreadyExists:      api.ErrorCodeAlreadyExists,
	errorx.KindPermissionDenied:   api.ErrorCodePermissionDenied,
	errorx.KindResourceExhausted:  api.ErrorCodeResourceExhausted,
	errorx.KindFailedPrecondition: api.ErrorCodeFailedPrecondition,
	errorx.KindAborted:            api.ErrorCodeAborted,
	errorx.KindOutOfRange:         api.ErrorCodeOutOfRange,
	errorx.KindUnimplemented:      api.ErrorCodeUnimplemented,
	errorx.KindInternal:           api.ErrorCodeInternal,
	errorx.KindUnavailable:        api.ErrorCodeUnavailable,
	errorx.KindDataLoss:           api.ErrorCodeDataLoss,
	errorx.KindUnauthenticated:    api.ErrorCodeUnauthenticated,
	errorx.KindRevisionConflict:   api.ErrorCodeRevisionConflict,
}

// KindToCode walks the error chain, finds the first errorx.Error,
// and produces an *api.ErrorStatusCode. The wire `code` is the
// gRPC-style short identifier for the kind; the original message
// is preserved in `message` so operators can diagnose. Non-errorx
// errors collapse to KindInternal → 500 / "internal".
//
// Exported so the ogen WithErrorHandler fallback (handler.NewError)
// can reuse the same logic for the SecurityError path that
// bypasses the chain.
func KindToCode(err error) *api.ErrorStatusCode {
	if err == nil {
		return nil
	}
	kind := errorx.KindOf(err)
	code, ok := kindToCode[kind]
	if !ok {
		code = api.ErrorCodeInternal
	}
	msg := err.Error()
	var de errorx.Error
	if errors.As(err, &de) {
		msg = fmt.Sprintf("%s: %s", de.Kind(), de.Error())
	}
	return &api.ErrorStatusCode{
		StatusCode: kind.Status(),
		Response: api.Error{
			Code:    code,
			Message: msg,
		},
	}
}

// ErrorMiddleware wraps handler-returned errors as *api.ErrorStatusCode
// so ogen's encoder writes the right status. Place BEFORE
// PanicMiddleware in the chain.
func ErrorMiddleware() middleware.Middleware {
	return func(req middleware.Request, next middleware.Next) (middleware.Response, error) {
		resp, err := next(req)
		if err == nil {
			return resp, nil
		}
		esc := KindToCode(err)
		if esc == nil {
			return resp, nil
		}
		// ogen's `errors.Into[*ErrorStatusCode](err)` walks the
		// chain — passing the bare ESC is not enough because the
		// generated dispatch in oas_handlers_gen.go expects a
		// wrapped error. Wrap with %w so the chain contains it.
		return resp, fmt.Errorf("apiserver: %w", esc)
	}
}

// PanicMiddleware catches handler panics and turns them into a
// 503 ErrorStatusCode. Place AFTER ErrorMiddleware so a panic from
// a later middleware still gets caught.
func PanicMiddleware() middleware.Middleware {
	return func(req middleware.Request, next middleware.Next) (resp middleware.Response, err error) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			logx.Error(req.Context,
				errorx.New(errorx.KindUnavailable, "middleware: handler panic"),
				"apiserver.panic_recovered",
				slog.String("operation", req.OperationName),
				slog.Any("panic_value", r),
				slog.String("stack", string(debug.Stack())),
			)
			err = KindToCode(errorx.New(errorx.KindUnavailable, "internal panic"))
		}()
		return next(req)
	}
}
