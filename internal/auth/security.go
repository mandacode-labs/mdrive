package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type contextKey string

const sessionKey contextKey = "session"

// SecurityHandler implements ogen's SecurityHandler for cookie and bearer auth.
type SecurityHandler struct {
	auth       *Service
	cookieName string
}

func NewSecurityHandler(auth *Service) *SecurityHandler {
	return &SecurityHandler{auth: auth, cookieName: SessionCookieName}
}

func (s *SecurityHandler) HandleBearerAuth(ctx context.Context, _ api.OperationName, t api.BearerAuth) (context.Context, error) {
	if SessionFromContext(ctx) != nil {
		return ctx, nil
	}
	sess, err := s.auth.store.Get(ctx, t.Token)
	if err != nil {
		return ctx, fmt.Errorf("bearer session: %w", err)
	}
	return ContextWithSession(ctx, sess), nil
}

func (s *SecurityHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if the request already presents a bearer token; ogen will handle it.
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}

		sess, err := s.extractSessionFromCookie(r)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Make the session available in context and synthesize an Authorization
		// header so the generated ogen security handler validates it normally.
		r = r.WithContext(ContextWithSession(r.Context(), sess))
		r.Header.Set("Authorization", "Bearer "+sess.ID)
		next.ServeHTTP(w, r)
	})
}

func (s *SecurityHandler) extractSessionFromCookie(r *http.Request) (*session.Session, error) {
	cookie, err := r.Cookie(s.cookieName)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("no session cookie")
	}
	return s.auth.store.Get(r.Context(), cookie.Value)
}

func SessionFromContext(ctx context.Context) *session.Session {
	sess, ok := ctx.Value(sessionKey).(*session.Session)
	if !ok {
		return nil
	}
	return sess
}

func UserIDFromContext(ctx context.Context) string {
	sess := SessionFromContext(ctx)
	if sess == nil {
		return ""
	}
	return sess.UserID
}

func ContextWithSession(ctx context.Context, s *session.Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

var _ api.SecurityHandler = (*SecurityHandler)(nil)
