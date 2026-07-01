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

// newCapturingLog builds a JSON logger wired with the bootstrap
// handler, registers it as slog.Default for the test, and
// restores the previous default on cleanup. Every
// logx.Info/Warn/Error/Request call reads slog.Default, so the
// buffer captures every log line.
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

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-abc")
	assert.Equal(t, "req-abc", RequestIDFromContext(ctx))
	assert.Equal(t, "", RequestIDFromContext(context.Background()))
}

func TestUserIDRoundTrip(t *testing.T) {
	ctx := WithUserID(context.Background(), "user-1")
	assert.Equal(t, "user-1", UserIDFromContext(ctx))
	assert.Equal(t, "", UserIDFromContext(context.Background()))
}

func TestWithRequestIDEmptyIsNoop(t *testing.T) {
	parent := WithRequestID(context.Background(), "req-1")
	child := WithRequestID(parent, "")
	assert.Equal(t, "req-1", RequestIDFromContext(child),
		"empty id must not clobber an existing one")
}

func TestErrorMapsKindToStatusAndLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	ctx := WithRequestID(context.Background(), "req-1")
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

	ctx := WithRequestID(context.Background(), "req-2")
	Error(ctx, errorx.New(errorx.KindServiceDegraded, "boom"), "test")

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
	assert.NotEmpty(t, line["error_type"], "raw errors must record their concrete type")
	assert.Equal(t, "ERROR", line["level"])
}

func TestErrorIncludesUserIDFromCtx(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	ctx := WithUserID(context.Background(), "user-42")
	Error(ctx, errorx.New(errorx.KindForbidden, "denied"), "perm check")

	line := parseLine(t, buf.String())
	assert.Equal(t, "user-42", line["user_id"],
		"user_id stored in ctx must appear on error logs")
}

func TestRequestLevelsByStatus(t *testing.T) {
	for _, tt := range []struct {
		status int
		level  string
	}{
		{200, "INFO"},
		{302, "INFO"},
		{400, "WARN"},
		{404, "WARN"},
		{500, "ERROR"},
	} {
		buf := &bytes.Buffer{}
		newCapturingLog(t, buf)
		Request(context.Background(), "GET", "/x", tt.status, 12)
		line := parseLine(t, buf.String())
		assert.Equal(t, tt.level, line["level"],
			"status %d must log %s", tt.status, tt.level)
	}
}

func TestRequestSkipsHealth(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)
	Request(context.Background(), "GET", "/health", 200, 5)
	assert.Empty(t, buf.String(), "health checks must not be logged")
}

func TestRequestIncludesRequestID(t *testing.T) {
	buf := &bytes.Buffer{}
	newCapturingLog(t, buf)

	ctx := WithRequestID(context.Background(), "req-9")
	Request(ctx, "GET", "/api/x", 200, 7)
	line := parseLine(t, buf.String())
	assert.Equal(t, "req-9", line["request_id"])
	assert.Equal(t, "GET", line["method"])
	assert.Equal(t, "/api/x", line["path"])
	assert.Equal(t, float64(200), line["status"])
	assert.Equal(t, float64(7), line["duration_ms"])
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
}
