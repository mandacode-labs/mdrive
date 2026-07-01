package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/logx"
)

// TestBootstrapLoggerInjectsRequestID proves the production
// logger built by logx.New surfaces the request_id stored in ctx
// on every emitted line, so operators can correlate any log
// entry to a response header without each call site having to
// remember RequestIDFromContext.
func TestBootstrapLoggerInjectsRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	logx.New(logx.Config{Env: "test", Level: "debug", Format: "json", Writer: buf})
	log := slog.Default()

	ctx := logx.WithRequestID(context.Background(), "req-abc")
	log.InfoContext(ctx, "hello")

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "req-abc", line["request_id"],
		"bootstrap logger must pull the request_id from ctx")
}

// TestBootstrapLoggerInjectsUserID proves user_id flows through
// the same path, so an operator can filter logs by authenticated
// user without changing any call site.
func TestBootstrapLoggerInjectsUserID(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	logx.New(logx.Config{Env: "test", Level: "debug", Format: "json", Writer: buf})
	log := slog.Default()

	ctx := logx.WithUserID(context.Background(), "user-7")
	log.InfoContext(ctx, "hi")

	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "user-7", line["user_id"])
}

// TestBootstrapLoggerEmptyIDSkipsAttr covers the no-request-id /
// no-user-id path (e.g. a log line emitted before the request
// middleware runs, or for an anonymous request).
func TestBootstrapLoggerEmptyIDSkipsAttr(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	logx.New(logx.Config{Env: "test", Level: "debug", Format: "json", Writer: buf})
	log := slog.Default()

	log.InfoContext(context.Background(), "no ctx id")

	out := buf.String()
	assert.NotContains(t, out, `"request_id"`,
		"empty request_id must not emit a request_id key")
	assert.NotContains(t, out, `"user_id"`,
		"empty user_id must not emit a user_id key")
}
