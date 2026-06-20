// Package logging provides a structured logger based on zerolog.
package logging

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logger wraps zerolog.Logger with convenience helpers.
type Logger struct {
	zl zerolog.Logger
}

// NewLogger creates a new logger at the given level.
func NewLogger(env, level string) *Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(parseLevel(level))

	logger := log.With().Str("env", env).Logger()
	if env != "production" {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	return &Logger{zl: logger}
}

// L returns the underlying zerolog logger.
func (l *Logger) L() *zerolog.Logger {
	return &l.zl
}

// Convenience methods.
func (l *Logger) Info() *zerolog.Event  { return l.zl.Info() }
func (l *Logger) Warn() *zerolog.Event  { return l.zl.Warn() }
func (l *Logger) Error() *zerolog.Event { return l.zl.Error() }
func (l *Logger) Debug() *zerolog.Event { return l.zl.Debug() }
func (l *Logger) Fatal() *zerolog.Event { return l.zl.Fatal() }

// Ctx returns the logger with a context.
func (l *Logger) Ctx() zerolog.Context {
	return l.zl.With()
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
