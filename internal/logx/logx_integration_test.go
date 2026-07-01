package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// TestNewProducesValidJSONOutput verifies the production logger
// emits parseable JSON when Format=json (the default in
// production). A regression here would break the log
// aggregator's parser in the same commit.
func TestNewProducesValidJSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	log := New(Config{Env: "production", Level: "info", Format: "json", Writer: buf})
	require.NotNil(t, log)
	log.Info("hello", slog.String("k", "v"))
	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "hello", line["msg"])
	assert.Equal(t, "v", line["k"])
	assert.Equal(t, "production", line["env"])
	assert.Equal(t, "mdrive", line["service"],
		"production env must tag the service for log aggregators")
}

func TestNewTextFormatEmitsKeyValue(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	log := New(Config{Env: "development", Level: "info", Format: "text", Writer: buf})
	log.Info("hello", slog.String("k", "v"))
	out := buf.String()
	assert.Contains(t, out, "msg=hello")
	assert.Contains(t, out, "k=v")
	assert.NotContains(t, out, `"`,
		"text format must not include JSON quotes")
}

func TestNewDefaultEnvForcesJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	log := New(Config{Env: "production", Writer: buf})
	log.Info("hello")
	out := buf.String()
	assert.True(t, bytes.HasPrefix([]byte(out), []byte("{")),
		"production env must default to JSON even with no Format: %q", out)
}

func TestNewDefaultNonProdUsesText(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	log := New(Config{Env: "test", Writer: buf})
	log.Info("hello")
	out := buf.String()
	assert.NotContains(t, out, `"msg"`,
		"non-production env must default to text format: %q", out)
}

func TestNewLevelFilterRespected(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	log := New(Config{Env: "test", Level: "warn", Format: "json", Writer: buf})
	log.Info("dropped")
	log.Warn("kept")
	out := buf.String()
	assert.NotContains(t, out, "dropped", "info must be dropped at warn level")
	assert.Contains(t, out, "kept")
}

// TestErrorIncludesStackAsAttribute proves the 5xx error path
// still emits a parseable stack attribute. The Operator uses
// this to find the source of panics in production without
// re-running with -tags=tracing.
func TestErrorIncludesStackAsAttribute(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	New(Config{Env: "test", Level: "debug", Format: "json", Writer: buf})

	Error(context.Background(),
		errorx.New(errorx.KindServiceDegraded, "boom"),
		"explode",
	)
	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	stack, ok := line["stack"].(string)
	require.True(t, ok, "5xx must include stack as string attr")
	assert.True(t, len(stack) > 100, "stack must be non-trivial")
}

// TestRequestIDEmptyAndSetIdempotent exercises the no-op paths
// in WithRequestID/WithUserID so a regression that turned
// empty strings into "no id" sentinels would be caught.
func TestRequestIDEmptyAndSetIdempotent(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-1")
	ctx = WithRequestID(ctx, "") // must not clobber
	assert.Equal(t, "req-1", RequestIDFromContext(ctx))

	ctx = WithUserID(context.Background(), "user-1")
	ctx = WithUserID(ctx, "")
	assert.Equal(t, "user-1", UserIDFromContext(ctx))
}

// TestClassifyRawErrorFallsBackToInternal makes the contract
// for non-errorx errors explicit: status 500, kind "unknown",
// error_type set. Anything else would silently produce a
// different HTTP status than the operator expects.
func TestClassifyRawErrorFallsBackToInternal(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	New(Config{Env: "test", Format: "json", Writer: buf})

	Error(context.Background(), errors.New("plain"), "x")
	var line map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, float64(500), line["status"])
	assert.Equal(t, "unknown", line["kind"])
	assert.Equal(t, "*errors.errorString", line["error_type"])
}

// TestErrorAndInfoOnEmptyCtx is a guard rail: a bare
// context.Background still produces log lines via slog.Default
// (the configured buffer in this test). Reaching here without
// panic is the assertion.
func TestErrorAndInfoOnEmptyCtx(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	New(Config{Env: "test", Format: "json", Writer: buf})

	Error(context.Background(), errors.New("x"), "msg")
	Info(context.Background(), "hello")
}
