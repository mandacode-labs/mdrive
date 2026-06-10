package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/mandacode-labs/mdrive/internal/logging"
)

// RecoveryMiddleware recovers from panics and logs them.
func RecoveryMiddleware(logger *logging.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rv := recover(); rv != nil {
					logging.Ctx(r.Context()).Error().
						Interface("panic", rv).
						Str("stack", string(debug.Stack())).
						Msg("panic recovered")

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
