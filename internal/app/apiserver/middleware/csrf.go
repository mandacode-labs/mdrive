package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// safeMethods are HTTP methods that do not change state and
// therefore do not need CSRF protection.
var safeMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodOptions: {},
	http.MethodTrace:   {},
}

// CSRFConfig configures the CSRFMiddleware.
type CSRFConfig struct {
	// AllowedOrigins is the set of origins permitted to make
	// state-changing requests. Comparison is scheme+host(+port),
	// case-insensitive.
	AllowedOrigins []string
}

// CSRFMiddleware validates the Origin (and Referer as fallback) on
// state-changing requests against an allowlist. Cookie auth + Origin
// is the chosen posture, not bearer tokens.
func CSRFMiddleware(cfg CSRFConfig) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[normalizeOrigin(o)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, safe := safeMethods[r.Method]; safe {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = refererOrigin(r.Header.Get("Referer"))
			}
			if origin == "" {
				csrfReject(w, "missing origin")
				return
			}
			if _, ok := allowed[normalizeOrigin(origin)]; !ok {
				csrfReject(w, "origin not allowed")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func csrfReject(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "csrf: " + reason})
}

func normalizeOrigin(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}

func refererOrigin(referer string) string {
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
