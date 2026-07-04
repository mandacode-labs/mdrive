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

// KindToCode walks the error chain, finds the first errorx.Error,
// and produces an *api.ErrorStatusCode. The wire `code` is always
// `internal`; the kind and original message live in `message` so
// clients can branch on them without depending on a closed enum.
// Non-errorx errors collapse to 500. Exported so the ogen
// WithErrorHandler fallback (handler.NewError) can reuse the
// same logic for the SecurityError path that bypasses the chain.
func KindToCode(err error) *api.ErrorStatusCode {
	if err == nil {
		return nil
	}
	var de errorx.Error
	kind := errorx.KindOf(err)
	msg := err.Error()
	if errors.As(err, &de) {
		msg = fmt.Sprintf("%s: %s", de.Kind(), de.Error())
	}
	return &api.ErrorStatusCode{
		StatusCode: kind.Status(),
		Response: api.Error{
			Code:    api.ErrorCodeInternal,
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
				errorx.New(errorx.KindServiceDegraded, "middleware: handler panic"),
				"apiserver.panic_recovered",
				slog.String("operation", req.OperationName),
				slog.Any("panic_value", r),
				slog.String("stack", string(debug.Stack())),
			)
			err = KindToCode(errorx.New(errorx.KindServiceDegraded, "internal panic"))
		}()
		return next(req)
	}
}
