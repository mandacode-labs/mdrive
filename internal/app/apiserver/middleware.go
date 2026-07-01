package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

const requestIDHeader = "X-Request-Id"

// RequestIDMiddleware ensures every request has a request ID in its
// context. Inbound X-Request-Id is reused when present, otherwise a new
// 16-byte hex ID is generated. The ID is echoed back in the response
// header and stored in the context for downstream handlers and logs.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			generated, err := randomHex(8)
			if err != nil {
				http.Error(w, "request id unavailable", http.StatusInternalServerError)
				return
			}
			id = generated
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored in ctx, or "" if
// none is present.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// statusRecorder captures the response status code and byte count
// without buffering the body, so downstream middleware (logging)
// can observe what was written.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.status = http.StatusOK
		r.wroteHeader = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// withRequestLog logs every request after the handler chain finishes.
// Logs include method, path, status, duration, and request ID so any
// operator can correlate a user-reported error to a backend trace.
// Mount this outside the CORS middleware so OPTIONS preflights and
// panic-recovered 500s are also recorded.
func withRequestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		status := rec.status
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}
		log.LogAttrs(r.Context(), level, "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.String("request_id", RequestIDFromContext(r.Context())),
			slog.String("remote", r.RemoteAddr),
		)
	})
}

// recoverPanic catches panics raised inside the handler chain and
// converts them to a 500 response with a full stack trace logged.
// Without this, Go's net/http default recover silently returns an
// empty 500 with no log line, which is exactly the failure mode that
// hid the original /auth/me 500 from operators.
// Mount this as the OUTERMOST wrapper so every panic anywhere in the
// stack is captured and logged before the response is written.
func recoverPanic(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.ErrorContext(r.Context(), "panic recovered",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("request_id", RequestIDFromContext(r.Context())),
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)
				WriteError(w, fmt.Errorf("internal panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}