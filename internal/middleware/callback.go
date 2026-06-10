package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/utils"
)

type CallbackConfig struct {
	Secure      bool
	TTL         int
	CookieName  string
	FrontendURL string
	Domain      string
	SameSite    string
}

// CallbackMiddleware handles OAuth callback (set cookie + redirect) and logout (clear cookie).
func CallbackMiddleware(cfg *CallbackConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		parsedSameSite := parseSameSite(cfg.SameSite)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/auth/logout" {
				// Extract session_id from cookie into context
				// (security handler doesn't run for this endpoint)
				if c, err := r.Cookie(cfg.CookieName); err == nil && c.Value != "" {
					r = r.WithContext(utils.ContextWithSession(r.Context(), c.Value))
				}

				// #nosec G124 — Secure and SameSite are explicitly configured via config
				cookie := &http.Cookie{
					Name:     cfg.CookieName,
					Value:    "",
					Path:     "/",
					HttpOnly: true,
					Secure:   cfg.Secure,
					MaxAge:   -1,
					SameSite: parsedSameSite,
				}
				if cfg.Domain != "" {
					cookie.Domain = cfg.Domain
				}
				http.SetCookie(w, cookie)
				next.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodGet && r.URL.Path == "/auth/callback" {
				rec := &responseRecorder{ResponseWriter: w}
				next.ServeHTTP(rec, r)

				if rec.statusCode == http.StatusOK && len(rec.body) > 0 {
					var resp struct {
						SessionID string `json:"sessionId"`
					}
					if err := json.Unmarshal(rec.body, &resp); err == nil && resp.SessionID != "" {
						// #nosec G124 — Secure and SameSite are explicitly configured via config
						cookie := &http.Cookie{
							Name:     cfg.CookieName,
							Value:    resp.SessionID,
							Path:     "/",
							HttpOnly: true,
							Secure:   cfg.Secure,
							MaxAge:   cfg.TTL,
							SameSite: parsedSameSite,
						}
						if cfg.Domain != "" {
							cookie.Domain = cfg.Domain
						}
						http.SetCookie(w, cookie)
						http.Redirect(w, r, cfg.FrontendURL, http.StatusFound)
						return
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(rec.statusCode)
				_, _ = w.Write(rec.body)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseSameSite(sameSite string) http.SameSite {
	switch strings.ToLower(sameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "none", "":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	r.body = append(r.body, b...)
	return len(b), nil
}

func ProvideCallbackConfig(cfg *config.Config) *CallbackConfig {
	return &CallbackConfig{
		Secure:      cfg.Auth.Session.Secure,
		TTL:         cfg.Auth.Session.TTL,
		CookieName:  cfg.Auth.Session.CookieName,
		FrontendURL: cfg.Auth.Session.FrontendURL,
		Domain:      cfg.Auth.Session.Domain,
		SameSite:    cfg.Auth.Session.SameSite,
	}
}
