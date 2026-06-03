package middleware

import (
	"net/http"

	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/session"
	"github.com/mandacode-labs/retrowin-go/internal/utils"
)

const (
	// SessionCookieName is the name of the session cookie.
	SessionCookieName = "retrowin_session"
)

// SessionAuth holds session authentication configuration.
type SessionAuth struct {
	sessionSvc session.SessionService
	secure     bool
}

// SessionAuthConfig holds session authentication configuration.
type SessionAuthConfig struct {
	SessionService session.SessionService
	Secure         bool
}

// NewSessionAuth creates a new SessionAuth middleware.
func NewSessionAuth(cfg *SessionAuthConfig) *SessionAuth {
	return &SessionAuth{
		sessionSvc: cfg.SessionService,
		secure:     cfg.Secure,
	}
}

// RequireSession middleware validates the session cookie and adds user info to context.
func (a *SessionAuth) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			WriteError(w, errors.Unauthorized("missing session cookie"))
			return
		}

		sess, err := a.sessionSvc.Validate(r.Context(), session.SessionID(cookie.Value))
		if err != nil {
			WriteError(w, errors.Unauthorized("invalid or expired session"))
			return
		}

		// Add session info to context
		ctx := r.Context()
		ctx = utils.ContextWithUserID(ctx, sess.UserID())

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetSessionCookie sets the session cookie.
func SetSessionCookie(w http.ResponseWriter, sessionID string, secure bool) {
	// #nosec G124 — Secure and SameSite are explicitly configured via parameter
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie clears the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	// #nosec G124 — Secure and SameSite are explicitly configured via parameter
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		MaxAge:   -1,
	})
}
