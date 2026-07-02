package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func newCapturingLog(t *testing.T, buf *bytes.Buffer) *slog.Logger {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	return New(Config{Env: "test", Level: "debug", Format: "json", Writer: buf})
}

func parseLine(t *testing.T, raw string) map[string]any {
	t.Helper()
	var line map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(raw)), &line),
		"log line must be valid JSON: %q", raw)
	return line
}

func TestWithLoggerFromContextRoundTrip(t *testing.T) {
	buf := &bytes.Buffer{}
	log := newCapturingLog(t, buf)
	ctx := WithLogger(context.Background(), log)
	assert.Same(t, log, FromContext(ctx))
	assert.Same(t, slog.Default(), FromContext(context.Background()),
		"missing ctx value must fall back to slog.Default")
}

func TestInfoWarnDebugLevels(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	Info(context.Background(), "info msg", slog.String("k", "v"))
	Warn(context.Background(), "warn msg", slog.String("k", "v"))
	Debug(context.Background(), "debug msg", slog.String("k", "v"))

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "INFO", parseLine(t, lines[0])["level"])
	assert.Equal(t, "WARN", parseLine(t, lines[1])["level"])
	assert.Equal(t, "DEBUG", parseLine(t, lines[2])["level"])
	assert.Equal(t, "v", parseLine(t, lines[0])["k"])
}

func TestErrorMapsKindToStatusAndLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	ctx := WithLogger(context.Background(), slog.Default().With("request_id", "req-1"))
	Error(ctx, errorx.New(errorx.KindBadRequest, "bad input"), "test")

	line := parseLine(t, buf.String())
	assert.Equal(t, "WARN", line["level"], "4xx must be WARN")
	assert.Equal(t, float64(400), line["status"])
	assert.Equal(t, "bad_request", line["kind"])
	assert.Equal(t, "req-1", line["request_id"])
	assert.Equal(t, "bad input", line["err"])
	_, hasStack := line["stack"]
	assert.False(t, hasStack, "non-5xx must not carry stack trace")
}

func TestErrorAt5xxCarriesStack(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	Error(context.Background(), errorx.New(errorx.KindServiceDegraded, "boom"), "test")

	line := parseLine(t, buf.String())
	assert.Equal(t, "ERROR", line["level"], "5xx must be ERROR")
	assert.Equal(t, float64(503), line["status"])
	stack, ok := line["stack"].(string)
	require.True(t, ok, "5xx must include stack trace as string")
	assert.True(t, strings.HasPrefix(stack, "goroutine "), "stack must look like a runtime stack")
}

func TestErrorFallsBackTo500ForRawError(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	Error(context.Background(), errors.New("raw"), "test")

	line := parseLine(t, buf.String())
	assert.Equal(t, float64(500), line["status"])
	assert.Equal(t, "unknown", line["kind"])
	assert.Equal(t, "ERROR", line["level"])
}

func TestErrorExtraAttrsAreMerged(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	Error(context.Background(),
		errorx.New(errorx.KindForbidden, "denied"),
		"perm check",
		slog.String("user_id", "u1"),
		slog.String("resource", "drive"),
	)
	line := parseLine(t, buf.String())
	assert.Equal(t, "u1", line["user_id"])
	assert.Equal(t, "drive", line["resource"])
}

func TestNewEnvAndServiceAttrs(t *testing.T) {
	buf := &bytes.Buffer{}
	New(Config{Env: "production", Level: "info", Format: "json", Writer: buf})
	Info(context.Background(), "hi")

	line := parseLine(t, buf.String())
	assert.Equal(t, "production", line["env"])
	assert.Equal(t, "mdrive", line["service"])
}

func TestBootstrapLoggerInjectsRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	New(Config{Env: "test", Level: "debug", Format: "json", Writer: buf})

	ctx := WithLogger(context.Background(), slog.Default().With("request_id", "req-abc"))
	Info(ctx, "hello")

	line := parseLine(t, buf.String())
	assert.Equal(t, "req-abc", line["request_id"],
		"ctx logger must surface its request_id on every line")
}

func TestBootstrapLoggerInjectsUserID(t *testing.T) {
	buf := &bytes.Buffer{}
	New(Config{Env: "test", Level: "debug", Format: "json", Writer: buf})

	ctx := WithLogger(context.Background(), slog.Default().With("user_id", "user-7"))
	Info(ctx, "hi")

	line := parseLine(t, buf.String())
	assert.Equal(t, "user-7", line["user_id"])
}

func TestBootstrapLoggerEmptyIDSkipsAttr(t *testing.T) {
	buf := &bytes.Buffer{}
	New(Config{Env: "test", Level: "debug", Format: "json", Writer: buf})

	Info(context.Background(), "no ctx id")

	out := buf.String()
	assert.NotContains(t, out, `"request_id"`,
		"slog.Default must not carry a request_id key")
	assert.NotContains(t, out, `"user_id"`,
		"slog.Default must not carry a user_id key")
}

func TestNewTextFormatForNonProduction(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	New(Config{Env: "development", Level: "info", Format: "", Writer: buf})
	Info(context.Background(), "hi", slog.String("k", "v"))
	assert.Contains(t, buf.String(), "k=v", "text format must emit key=value")
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo,
		"junk":  slog.LevelInfo,
	}
	for s, want := range cases {
		assert.Equal(t, want, parseLevel(s), "parseLevel(%q)", s)
	}
}
