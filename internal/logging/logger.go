// Package logging provides structured logging with zerolog.
package logging

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger is the application logger interface.
type Logger struct {
	logger zerolog.Logger
}

// NewLogger creates a new logger with the given configuration.
func NewLogger(env, level string) *Logger {
	// Parse log level
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(lvl)

	var zl zerolog.Logger
	if env == "development" {
		// Pretty console output for development
		zl = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		})
	} else {
		// JSON output for production
		zl = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	return &Logger{logger: zl}
}

// WithContext returns a logger with context values.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{logger: l.logger.With().Ctx(ctx).Logger()}
}

// WithStr returns a logger with a string field.
func (l *Logger) WithStr(key, val string) *Logger {
	return &Logger{logger: l.logger.With().Str(key, val).Logger()}
}

// WithInt returns a logger with an int field.
func (l *Logger) WithInt(key string, val int) *Logger {
	return &Logger{logger: l.logger.With().Int(key, val).Logger()}
}

// WithError returns a logger with an error field.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{logger: l.logger.With().Err(err).Logger()}
}

// Debug logs a debug message.
func (l *Logger) Debug() *zerolog.Event {
	return l.logger.Debug()
}

// Info logs an info message.
func (l *Logger) Info() *zerolog.Event {
	return l.logger.Info()
}

// Warn logs a warning message.
func (l *Logger) Warn() *zerolog.Event {
	return l.logger.Warn()
}

// Error logs an error message.
func (l *Logger) Error() *zerolog.Event {
	return l.logger.Error()
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal() *zerolog.Event {
	return l.logger.Fatal()
}

// Ctx extracts the logger from context or returns the default logger.
func Ctx(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey{}).(*Logger); ok {
		return l
	}
	return &Logger{logger: log.Logger}
}

// WithLogger adds the logger to the context.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

type loggerKey struct{}

// HTTPRequest logs an HTTP request.
func (l *Logger) HTTPRequest(method, path string, status, durationMs int, err error) {
	event := l.logger.Info().
		Str("method", method).
		Str("path", path).
		Int("status", status).
		Int("duration_ms", durationMs)

	if err != nil {
		event = event.Err(err)
	}

	event.Msg("http_request")
}
