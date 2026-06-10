package middleware

import (
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/internal/logging"
)

// RequestLoggingMiddleware logs all HTTP requests with timing and status.
func RequestLoggingMiddleware(logger *logging.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &logResponseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			// Add logger to context
			ctx := logging.WithLogger(r.Context(), logger)
			r = r.WithContext(ctx)

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.HTTPRequest(
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				int(duration.Milliseconds()),
				nil,
			)
		})
	}
}

// logResponseRecorder wraps http.ResponseWriter to capture status code.
type logResponseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rr *logResponseRecorder) WriteHeader(code int) {
	if !rr.written {
		rr.statusCode = code
		rr.written = true
		rr.ResponseWriter.WriteHeader(code)
	}
}

func (rr *logResponseRecorder) Write(b []byte) (int, error) {
	if !rr.written {
		rr.written = true
	}
	return rr.ResponseWriter.Write(b)
}
