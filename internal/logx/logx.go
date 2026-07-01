// Package logx provides standardized structured logging across mdrive.
//
// It owns three things:
//
//   1. A request-ID context key (mirror of the one the apiserver
//      middleware writes). Any package that wants to log with
//      request correlation pulls it through Context, never through
//      global state.
//
//   2. Error, which logs an error at the level implied by its
//      HTTP status (5xx -> ERROR with stack trace, 4xx -> WARN,
//      else -> INFO). The errorx package owns the status mapping;
//      logx only reflects it.
//
//   3. Request, which logs a single HTTP access entry. /health is
//      excluded by default because k8s probes generate steady
//      noise that drowns out real traffic in operator dashboards.
//
// All log lines are emitted through a *slog.Logger configured by
// the caller; logx itself has no logger state.
package logx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

type ctxKey int

const ctxKeyRequestID ctxKey = iota

// WithRequestID stores a request ID in ctx so every downstream
// log line emitted via this package can be correlated.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestIDFromContext returns the request ID stored in ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// Error logs err using errorx to determine the HTTP status and the
// log level. msg is the user-facing log message (e.g. "handler
// error"). The error message itself is always emitted under
// "error"; the kind string and HTTP status come from errorx.
//
// 5xx errors include a stack trace under "stack" so the operator
// can locate the source without re-running with -tags=tracing.
// 4xx errors are operator-relevant (auth failure, bad request)
// but expected, hence WARN. 2xx/3xx errors are uncommon here but
// logged at INFO for completeness.
//
// Extra attrs are merged into the log entry alongside the
// standard fields; pass them as slog.Attr values (slog.String,
// slog.Int, etc.) so the JSON layout stays sorted.
func Error(ctx context.Context, log *slog.Logger, err error, msg string, attrs ...slog.Attr) {
	status, kind := classify(err)
	level := levelForStatus(status)

	base := []slog.Attr{
		slog.String("error", err.Error()),
		slog.String("error_type", fmt.Sprintf("%T", err)),
		slog.String("kind", kind),
		slog.Int("status", status),
	}
	if id := RequestIDFromContext(ctx); id != "" {
		base = append(base, slog.String("request_id", id))
	}
	if level >= slog.LevelError {
		base = append(base, slog.String("stack", string(debug.Stack())))
	}
	log.LogAttrs(ctx, level, msg, append(base, attrs...)...)
}

// Request logs a single HTTP access entry. status < 500 -> INFO or
// WARN, status >= 500 -> ERROR. /health is excluded so probe
// traffic does not bury real requests.
func Request(ctx context.Context, log *slog.Logger, method, path string, status int, durationMS int64) {
	if path == "/health" {
		return
	}
	level := levelForStatus(status)
	attrs := []slog.Attr{
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Int64("duration_ms", durationMS),
	}
	if id := RequestIDFromContext(ctx); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	log.LogAttrs(ctx, level, "request", attrs...)
}

func classify(err error) (int, string) {
	var de errorx.Error
	if errors.As(err, &de) {
		return de.Kind().Status(), de.Kind().String()
	}
	return http.StatusInternalServerError, "unknown"
}

func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}