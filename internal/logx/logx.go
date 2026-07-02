// Package logx is a thin slog facade: it builds and installs the
// process-wide *slog.Logger, exposes ctx-scoped access to it, and
// surfaces the four level-specific entry points. The rest of the
// codebase only ever calls logx.{Info,Warn,Debug,Error}, so swapping
// the underlying engine (zerolog, otel, ...) means rewriting this
// one file.
package logx

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

type Config struct {
	Env    string
	Level  string
	Format string
	Writer io.Writer
}

type ctxKey struct{}

// New builds a *slog.Logger, registers it as slog.Default, and
// returns it. Format "" picks text for non-production, json otherwise.
func New(cfg Config) *slog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stderr
	}
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}
	format := cfg.Format
	if format == "" {
		if cfg.Env == "production" {
			format = "json"
		} else {
			format = "text"
		}
	}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	log := slog.New(h).With("env", cfg.Env)
	if cfg.Env == "production" {
		log = log.With("service", "mdrive")
	}
	slog.SetDefault(log)
	return log
}

// Info logs at INFO via the ctx logger.
func Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	FromContext(ctx).LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

// Warn logs at WARN via the ctx logger.
func Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	FromContext(ctx).LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

// Debug logs at DEBUG via the ctx logger.
func Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	FromContext(ctx).LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

// Error logs err at the level implied by errorx.Kind. 5xx includes stack.
func Error(ctx context.Context, err error, msg string, attrs ...slog.Attr) {
	kind := errorx.KindOf(err)
	status := kind.Status()
	level := levelForStatus(status)
	base := []slog.Attr{
		slog.String("err", err.Error()),
		slog.String("kind", kind.String()),
		slog.Int("status", status),
	}
	if level >= slog.LevelError {
		base = append(base, slog.String("stack", string(debug.Stack())))
	}
	FromContext(ctx).LogAttrs(ctx, level, msg, append(base, attrs...)...)
}

// WithLogger returns a ctx carrying log.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// FromContext returns the ctx logger, falling back to slog.Default.
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
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
