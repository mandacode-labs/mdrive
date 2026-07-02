package apiserver

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

const requestIDHeader = "X-Request-Id"

// RequestIDMiddleware ensures every request has a request ID stored
// in its context and echoed in the response header. Inbound
// X-Request-Id is reused when present; otherwise a new 16-hex-char
// ID is generated. The ID flows through the context via logx so
// every downstream log line can be correlated.
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
		next.ServeHTTP(w, r.WithContext(logx.WithRequestID(r.Context(), id)))
	})
}

// RequestIDFromContext returns the request ID stored in ctx, or "" if
// none is present. Kept as a thin re-export so callers in this
// package don't need to import logx.
func RequestIDFromContext(ctx interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}) string {
	return logx.RequestIDFromContext(ctx)
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

// withRequestLog logs every request after the handler chain
// finishes. /health is excluded so k8s probe traffic doesn't
// drown out real requests in the operator dashboard.
//
// Level is decided by status: 5xx -> ERROR, 4xx -> WARN, else ->
// INFO. request_id is always attached when present in ctx so
// log lines correlate with the response header. Emission goes
// through slog.Default (configured by logx.New at boot).
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		logx.Debug(r.Context(), "apiserver.request.enter",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		next.ServeHTTP(rec, r)
		logx.Request(r.Context(), r.Method, r.URL.Path, rec.status, time.Since(start).Milliseconds())
		logx.Debug(r.Context(), "apiserver.request.exit",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Int("bytes", rec.bytes),
		)
	})
}

// recoverPanic catches panics raised inside the handler chain and
// converts them to a 500 response. The panic is wrapped in an
// errorx.KindServiceDegraded error and logged via logx.Error so
// the operator sees a single, consistent format (status, kind,
// request_id, stack).
//
// Mount this as the OUTERMOST wrapper so every panic anywhere in
// the stack is captured before the response is written.
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

// randomHex returns a hex-encoded string of n random bytes.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
