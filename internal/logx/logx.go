// Package logx is the single entry point for structured logging
// across mdrive.
//
// It owns three things:
//
//  1. Context-scoped correlation. WithRequestID / WithUserID
//     store IDs in ctx; the bootstrap handler reads them on
//     every line so a single log entry is enough to correlate
//     a request, its operator, and its user.
//
//  2. Error, which logs an error at the level implied by its
//     HTTP status (5xx -> ERROR with stack trace, 4xx -> WARN,
//     else -> INFO). The errorx package owns the status mapping;
//     logx only reflects it.
//
//  3. Request, which logs a single HTTP access entry. /health is
//     excluded by default because k8s probes generate steady
//     noise that drowns out real traffic in operator dashboards.
//
// Production wires the configured logger as slog.Default at boot
// via New; every call site that already has a ctx can log via
// logx.Info / logx.Warn / logx.Debug / logx.Error without
// juggling a logger parameter.
package logx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

type attrKey struct{ name string }

var (
	requestIDKey = attrKey{name: "request_id"}
	userIDKey    = attrKey{name: "user_id"}
)

// WithRequestID stores a request ID in ctx so every downstream
// log line emitted via this package can be correlated.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request ID stored in ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// WithUserID stores an authenticated user ID in ctx. The Handler
// reads it on every log line so every entry attributed to an
// authenticated request can be filtered by user in the operator's
// dashboard.
func WithUserID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, userIDKey, id)
}

// UserIDFromContext returns the authenticated user ID stored in
// ctx, or "".
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// Info logs at INFO with the logger in ctx.
func Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	slog.Default().LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

// Warn logs at WARN.
func Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	slog.Default().LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

// Debug logs at DEBUG.
func Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	slog.Default().LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

// Error logs err using errorx to determine the HTTP status and the
// log level. msg is the user-facing log message (e.g. "handler
// error"). The error message itself is always emitted under "err"
// and again under "err_chain" (errorx.Error() renders the full
// chain as "outer: inner"); the kind string and HTTP status come
// from errorx.
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
func Error(ctx context.Context, err error, msg string, attrs ...slog.Attr) {
	status, kind := classify(err)
	level := levelForStatus(status)

	base := []slog.Attr{
		slog.String("err", err.Error()),
		slog.String("err_chain", err.Error()),
		slog.String("error_type", fmt.Sprintf("%T", err)),
		slog.String("kind", kind),
		slog.Int("status", status),
	}
	if level >= slog.LevelError {
		base = append(base, slog.String("stack", string(debug.Stack())))
	}
	slog.Default().LogAttrs(ctx, level, msg, append(base, attrs...)...)
}

// Request logs a single HTTP access entry. status < 500 -> INFO or
// WARN, status >= 500 -> ERROR. /health is excluded so probe
// traffic does not bury real requests.
func Request(ctx context.Context, method, path string, status int, durationMS int64) {
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
	slog.Default().LogAttrs(ctx, level, "request", attrs...)
}

// Config describes how New builds the production logger.
type Config struct {
	// Env is emitted as a base attr on every line (e.g.
	// "production", "development") so the operator dashboard
	// can filter by environment.
	Env string
	// Level is the minimum level emitted. One of "debug",
	// "info", "warn", "error" (case-insensitive). Empty
	// defaults to "info".
	Level string
	// Format is "json" or "text". Empty defaults to "json" in
	// production and "text" elsewhere.
	Format string
	// AddSource enables file:line in every line. Off by
	// default (5-15% throughput cost).
	AddSource bool
	// Writer is the sink. nil means os.Stderr.
	Writer io.Writer
}

// New builds a *slog.Logger wired with the bootstrap Handler
// (injects request_id and user_id from ctx) and registers it as
// slog.Default so callers outside the request scope (CLI
// subcommands, jobs) get a configured logger without explicit
// plumbing. The returned logger is tagged with the "env" base
// attr and (if env == "production") a stable "service" attr so
// log aggregators can group by environment and service.
//
// Tests use New directly with a buffer-backed Writer and
// t.Cleanup to restore the previous default.
func New(cfg Config) *slog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stderr
	}
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}
	var inner slog.Handler
	format := cfg.Format
	if format == "" {
		if cfg.Env == "production" {
			format = "json"
		} else {
			format = "text"
		}
	}
	switch format {
	case "text":
		inner = slog.NewTextHandler(w, opts)
	default:
		inner = slog.NewJSONHandler(w, opts)
	}
	log := slog.New(&handler{Handler: inner})
	log = log.With("env", cfg.Env)
	if cfg.Env == "production" {
		log = log.With("service", "mdrive")
	}
	slog.SetDefault(log)
	return log
}

// handler wraps an inner slog.Handler and injects request_id and
// user_id from ctx on every record. The wrapper preserves
// WithAttrs/WithGroup by re-wrapping the inner handler so derived
// loggers keep the context-injection behaviour.
type handler struct{ slog.Handler }

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if id := UserIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("user_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{Handler: h.Handler.WithGroup(name)}
}

func classify(err error) (int, string) {
	kind := errorx.KindOf(err)
	return kind.Status(), kind.String()
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

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
