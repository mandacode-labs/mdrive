// Package spec exposes the bundled OpenAPI 3.1 spec via a small
// http.Handler. Mount it on a mux — it is not a middleware.
package spec

import (
	"net/http"

	"github.com/mandacode-labs/mdrive/api"
)

// Spec is the bundled OpenAPI document.
var Spec = api.Spec

// Handler serves the embedded OpenAPI spec. Path is the mux's
// concern, not this handler's.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(Spec)
	})
}
