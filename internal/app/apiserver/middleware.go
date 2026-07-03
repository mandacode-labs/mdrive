package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/ogen-go/ogen/middleware"
)

const requestIDHeader = "X-Request-Id"

type requestIDKey struct{}

// RequestIDMiddleware ensures every request has a request ID echoed
// in the response header and a ctx logger tagged with it. Inbound
// X-Request-Id is reused when present; otherwise a new 16-hex-char
// ID is generated.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			var b [8]byte
			if _, err := rand.Read(b[:]); err != nil {
				http.Error(w, "request id unavailable", http.StatusInternalServerError)
				return
			}
			id = hex.EncodeToString(b[:])
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
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// statusRecorder captures the response status so the access log
// can report it. Pattern matches ogen's own codeRecorder: header
// is the only authoritative status; bare Write falls through to
// the wrapped writer, which net/http implicitly turns into
// WriteHeader(200).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// ogenPanicGuard is an ogen-level middleware that converts a
// handler panic into a KindServiceDegraded error. ogen then
// routes the error through WithErrorHandler, producing a proper
// 5xx JSON response — unlike the HTTP-level recoverPanic, which
// is limited to appending a body once status is already
// committed.
//
// Together the two form defense in depth: ogenPanicGuard handles
// handler panic with a clean status code, while recoverPanic
// (HTTP-level) catches anything that escapes ogen, including
// response-encoder panics and middleware-pipeline panics.
func ogenPanicGuard(req middleware.Request, next middleware.Next) (resp middleware.Response, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			stack := debug.Stack()
			logx.Error(req.Context, errorx.New(errorx.KindServiceDegraded, "handler panic"),
				"ogen handler panic",
				slog.String("operation", req.OperationName),
				slog.String("request_id", RequestIDFromContext(req.Context)),
				slog.Any("panic_value", rec),
				slog.String("stack", string(stack)),
			)
			err = errorx.New(errorx.KindServiceDegraded, "handler panic")
		}
	}()
	return next(req)
}

// withRequestLog logs a single access line per request. /health is
// excluded so k8s probes don't drown out real traffic. Status
// decides level: 5xx -> ERROR, 4xx -> WARN, else -> INFO.
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		if r.URL.Path == "/health" {
			return
		}
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		logx.FromContext(r.Context()).LogAttrs(r.Context(), levelForStatus(status), "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	})
}

// recoverPanic catches any panic that escaped ogenPanicGuard:
// response-encoder panics, middleware-pipeline panics. Status
// cannot be overwritten once committed, so the recovery body
// rides the original status — but the panic is still logged with
// its stack.
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			err := errorx.New(errorx.KindServiceDegraded, "internal panic")
			logx.Error(r.Context(), err, "panic recovered",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.Any("panic_value", rec),
				slog.String("stack", string(debug.Stack())),
			)
			WriteError(w, err)
		}()
		next.ServeHTTP(w, r)
	})
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

// withCORS sets CORS response headers and short-circuits OPTIONS
// preflights with 204 No Content. When cfg.Enabled is false the
// handler is a pass-through.
func withCORS(next http.Handler, cfg config.CORSConfig) http.Handler {
	if !cfg.Enabled {
		return next
	}

	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ", ")
	origins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		origins[o] = struct{}{}
	}
	allowAll := false
	if _, ok := origins["*"]; ok {
		allowAll = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			switch {
			case allowAll && !cfg.AllowCredentials:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case allowAll && cfg.AllowCredentials:
				w.Header().Set("Access-Control-Allow-Origin", origin)
			default:
				if _, ok := origins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", strconv.FormatBool(cfg.AllowCredentials))
			}
			if exposedHeaders != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
			}
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			if allowedHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}
			if cfg.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
