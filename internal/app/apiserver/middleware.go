package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

const requestIDHeader = "X-Request-Id"

// RequestIDMiddleware ensures every request has a request ID
// echoed in the response header and a ctx logger tagged with
// it. Inbound X-Request-Id is reused when present; otherwise a
// new 16-hex-char ID is generated.
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

		log := slog.Default().With("request_id", id)
		if uid := auth.UserIDFromContext(r.Context()); uid != "" {
			log = log.With("user_id", uid)
		}
		ctx := logx.WithLogger(r.Context(), log)
		ctx = context.WithValue(ctx, requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored in ctx by
// RequestIDMiddleware, or "" if none is set.
func RequestIDFromContext(ctx interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

type requestIDKey struct{}

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

// withRequestLog logs a single access line per request. /health is
// excluded so k8s probes don't drown out real traffic. Status
// decides level: 5xx -> ERROR, 4xx -> WARN, else -> INFO.
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		log := logx.FromContext(r.Context())
		log.DebugContext(r.Context(), "apiserver.request.enter",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		next.ServeHTTP(rec, r)
		if r.URL.Path != "/health" {
			log.LogAttrs(r.Context(), levelForStatus(rec.status), "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		}
		log.DebugContext(r.Context(), "apiserver.request.exit",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
		)
	})
}

// recoverPanic catches panics raised inside the handler chain and
// converts them to a 500 response.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			err := errorx.New(errorx.KindServiceDegraded,
				fmt.Sprintf("internal panic: %v", rec))
			logx.Error(r.Context(), err, "panic recovered",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
			WriteError(w, err)
		}()
		next.ServeHTTP(w, r)
	})
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func levelForStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
