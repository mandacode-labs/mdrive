package logx

import (
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func newCapturingLog() (*slog.Logger, *strings.Builder) {
	buf := &strings.Builder{}
	log := slog.New(slog.NewJSONHandler(safeWriter{buf}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return log, buf
}

type safeWriter struct{ b *strings.Builder }

func (w safeWriter) Write(p []byte) (int, error) { return w.b.Write(p) }

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-abc")
	assert.Equal(t, "req-abc", RequestIDFromContext(ctx))
	assert.Equal(t, "", RequestIDFromContext(context.Background()))
}

func TestErrorMapsKindToStatusAndLevel(t *testing.T) {
	log, buf := newCapturingLog()
	ctx := WithRequestID(context.Background(), "req-1")

	Error(ctx, log, errorx.New(errorx.KindBadRequest, "bad input"), "test")
	out := buf.String()

	assert.Contains(t, out, `"level":"WARN"`, "4xx must be WARN")
	assert.Contains(t, out, `"status":400`)
	assert.Contains(t, out, `"kind":"bad_request"`)
	assert.Contains(t, out, `"request_id":"req-1"`)
	assert.Contains(t, out, `"error":"bad input"`)
	assert.NotContains(t, out, `"stack"`, "non-5xx must not carry stack trace")
}

func TestErrorAt5xxCarriesStack(t *testing.T) {
	log, buf := newCapturingLog()
	ctx := WithRequestID(context.Background(), "req-2")

	Error(ctx, log, errorx.New(errorx.KindServiceDegraded, "boom"), "test")
	out := buf.String()

	assert.Contains(t, out, `"level":"ERROR"`, "5xx must be ERROR")
	assert.Contains(t, out, `"status":503`)
	assert.Contains(t, out, `"stack":"goroutine`, "5xx must include stack trace")
}

func TestErrorFallsBackTo500ForRawError(t *testing.T) {
	log, buf := newCapturingLog()

	Error(context.Background(), log, errors.New("raw"), "test")
	out := buf.String()

	assert.Contains(t, out, `"status":500`)
	assert.Contains(t, out, `"kind":"unknown"`)
	assert.Contains(t, out, `"error_type"`)
	assert.Contains(t, out, `"level":"ERROR"`)
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
		log, buf := newCapturingLog()
		Request(context.Background(), log, "GET", "/x", tt.status, 12)
		assert.Contains(t, buf.String(), `"level":"`+tt.level+`"`,
			"status %d must log %s", tt.status, tt.level)
	}
}

func TestRequestSkipsHealth(t *testing.T) {
	log, buf := newCapturingLog()
	Request(context.Background(), log, "GET", "/health", 200, 5)
	assert.Empty(t, buf.String(), "health checks must not be logged")
}

func TestRequestIncludesRequestID(t *testing.T) {
	log, buf := newCapturingLog()
	ctx := WithRequestID(context.Background(), "req-9")
	Request(ctx, log, "GET", "/api/x", 200, 7)
	assert.Contains(t, buf.String(), `"request_id":"req-9"`)
}

func TestErrorExtraAttrsAreMerged(t *testing.T) {
	log, buf := newCapturingLog()
	Error(context.Background(), log,
		errorx.New(errorx.KindForbidden, "denied"),
		"perm check",
		slog.String("user_id", "u1"),
		slog.String("resource", "drive"),
	)
	out := buf.String()
	assert.Contains(t, out, `"user_id":"u1"`)
	assert.Contains(t, out, `"resource":"drive"`)
}

// Ensure the logx package does not panic on nil request in a
// typical httptest flow -- this used to bite in early refactors.
func TestNilSafeHTTPDoc(t *testing.T) {
	req := httptest.NewRequest("GET", "/x", nil)
	ctx := WithRequestID(req.Context(), "req-nil")
	log, _ := newCapturingLog()
	Request(ctx, log, "GET", "/x", 200, 1)
}