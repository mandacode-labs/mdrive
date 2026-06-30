package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/pkg/api"
)

type contextKey string

const sessionKey contextKey = "session"

// SecurityHandler satisfies ogen's api.SecurityHandler contract.
// It is the bearer-token path: when the request already carries
// an Authorization header, the cookie-bridge middleware has
// already populated the context.
type SecurityHandler struct {
	auth *Service
}

func NewSecurityHandler(auth *Service) *SecurityHandler {
	return &SecurityHandler{auth: auth}
}

func (s *SecurityHandler) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	if SessionFromContext(ctx) != nil {
		return ctx, nil
	}
	return ctx, fmt.Errorf("auth: no session for bearer token")
}

// Middleware bridges cookie-based OIDC sessions to ogen's
// bearer-auth contract. If the request already carries an
// Authorization header, it is passed through unchanged. Otherwise
// the session cookie is read, the user is looked up, and a
// synthesized "Bearer <userID>" header plus a context-bound
// Session are attached for downstream handlers.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}
		sess, err := s.readSessionCookie(r)
		if err != nil || sess == nil {
			next.ServeHTTP(w, r)
			return
		}
		u, err := s.users.GetByProviderID(r.Context(), s.providerName, sess.Subject)
		if err != nil || u == nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx := ContextWithSession(r.Context(), &Session{
			ID:        sess.Subject,
			UserID:    u.ID(),
			Provider:  s.providerName,
			IsAdmin:   sess.IsAdmin,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.sessionTTL),
		})
		r = r.WithContext(ctx)
		r.Header.Set("Authorization", "Bearer "+u.ID())
		next.ServeHTTP(w, r)
	})
}

// SessionFromContext returns the Session attached by Middleware,
// or nil if the request is unauthenticated.
func SessionFromContext(ctx context.Context) *Session {
	sess, ok := ctx.Value(sessionKey).(*Session)
	if !ok {
		return nil
	}
	return sess
}

// UserIDFromContext returns the authenticated user's id, or "" if
// the request is unauthenticated.
func UserIDFromContext(ctx context.Context) string {
	if sess := SessionFromContext(ctx); sess != nil {
		return sess.UserID
	}
	return ""
}

// IsAdmin reports whether the authenticated principal holds the
// admin role. Returns false for unauthenticated requests.
func IsAdmin(ctx context.Context) bool {
	if sess := SessionFromContext(ctx); sess != nil {
		return sess.IsAdmin
	}
	return false
}

// ContextWithSession attaches a Session to ctx. Used by tests and
// any caller that wants to simulate an authenticated request.
func ContextWithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, sessionKey, sess)
}

var _ api.SecurityHandler = (*SecurityHandler)(nil)
