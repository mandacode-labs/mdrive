package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/logx"
)

// TestContextHandlerInjectsRequestID proves the bootstrap logger
// surfaces the request_id stored in ctx on every emitted line,
// so operators can correlate any log entry to a response header
// without each call site having to remember RequestIDFromContext.
func TestContextHandlerInjectsRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := slog.New(contextHandler{Handler: inner})

	ctx := logx.WithRequestID(context.Background(), "req-abc")
	log.InfoContext(ctx, "hello")

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "req-abc", line["request_id"],
		"contextHandler must pull the request_id from ctx")
}

// TestContextHandlerPropagatesWith verifies the handler is
// safe to use with slog.Logger.With: subsequent calls preserve
// the per-call context (i.e. request_id is still pulled from
// ctx, not the With args).
func TestContextHandlerPropagatesWith(t *testing.T) {
	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := slog.New(contextHandler{Handler: inner}).With("component", "test")

	ctx := logx.WithRequestID(context.Background(), "req-xyz")
	log.InfoContext(ctx, "hi")

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "req-xyz", line["request_id"])
	assert.Equal(t, "test", line["component"], "With attrs must survive")
}

// TestContextHandlerEmptyIDSkipsAttr covers the no-request-id path
// (e.g. a log line emitted before the request middleware runs).
func TestContextHandlerEmptyIDSkipsAttr(t *testing.T) {
	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := slog.New(contextHandler{Handler: inner})

	log.InfoContext(context.Background(), "no ctx id")

	out := buf.String()
	assert.False(t, strings.Contains(out, `"request_id"`),
		"empty id must not emit a request_id key")
}