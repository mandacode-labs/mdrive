package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLog returns a JSON logger backed by buf and registers it
// as slog.Default so the middleware's logx calls emit to the
// same buffer. The previous default is restored on test cleanup.
func newTestLog(t *testing.T) (*slog.Logger, *strings.Builder) {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	buf := &strings.Builder{}
	log := slog.New(slog.NewJSONHandler(io.MultiWriter(buf), &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(log)
	return log, buf
}

func TestRecoverPanicConvertsTo5xx(t *testing.T) {
	_, _ = newTestLog(t)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	recoverPanic(panicHandler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"panic surfaces as 503 (KindServiceDegraded), not a raw 500")
	var apiErr map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apiErr),
		"5xx body must be JSON, not empty")
	assert.Contains(t, apiErr, "code")
}

func TestRecoverPanicLogsStack(t *testing.T) {
	_, buf := newTestLog(t)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	recoverPanic(panicHandler).ServeHTTP(rec, req)

	logs := buf.String()
	assert.Contains(t, logs, "panic recovered", "log must record the panic")
	assert.Contains(t, logs, "kaboom", "log must include the panic value")
	assert.Contains(t, logs, "stack", "log must include the stack trace")
}

func TestWithRequestLogRecordsStatus(t *testing.T) {
	_, buf := newTestLog(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	srv := httptest.NewServer(withRequestLog(inner))
	defer srv.Close()

	resp, _ := http.Get(srv.URL)
	_ = resp.Body.Close()

	logs := buf.String()
	assert.Contains(t, logs, `"status":418`, "log must record 418 status")
	assert.Contains(t, logs, `"method":"GET"`, "log must record method")
	assert.Contains(t, logs, `"path":"`, "log must record path")
}

func TestWithRequestLogEscalatesLevelFor5xx(t *testing.T) {
	_, buf := newTestLog(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(withRequestLog(inner))
	defer srv.Close()

	resp, _ := http.Get(srv.URL)
	_ = resp.Body.Close()

	logs := buf.String()
	assert.Contains(t, logs, `"level":"ERROR"`, "5xx must log at ERROR")
}

func TestStatusRecorderCapturesExplicitHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec}
	sr.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, sr.status)
}

func TestStatusRecorderDefaultsTo200OnBareWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec}
	_, err := sr.Write([]byte("hi"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, sr.status, "bare Write implies 200")
}

func TestRequestIDPropagation(t *testing.T) {
	h := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestIDFromContext(r.Context())
		_, _ = w.Write([]byte(id))
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/x", nil)
	req.Header.Set("X-Request-Id", "incoming-id-123")

	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	assert.Equal(t, "incoming-id-123", resp.Header.Get(requestIDHeader),
		"inbound X-Request-Id must be echoed")
	assert.Equal(t, "incoming-id-123", string(body),
		"downstream handler must see the same id from context")
}

// TestChainNoLeak verifies that recoverPanic + withRequestLog work
// together for normal (non-panicking) responses.
func TestChainNoLeak(t *testing.T) {
	_, buf := newTestLog(t)

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	})
	chain := recoverPanic(withRequestLog(ok))
	srv := httptest.NewServer(chain)
	defer srv.Close()

	resp, _ := http.Get(srv.URL)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))
	assert.NotContains(t, buf.String(), "panic recovered",
		"normal request must not log panic")
	assert.Contains(t, buf.String(), `"status":200`)
}

// Ensure Error-logging handler emits a real Error JSON, not an
// empty body, so an operator always has something to read when a
// handler panic sneaks past recoverPanic.
func TestWriteErrorEmitsJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var apiErr map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apiErr))
	assert.Equal(t, "internal_error", apiErr["code"])
}

// context round-trip sanity check
func TestRequestIDFromContextEmpty(t *testing.T) {
	assert.Equal(t, "", RequestIDFromContext(context.Background()))
}