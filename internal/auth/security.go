package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
		sess, err := s.extractSession(r)
		if err == nil {
			r = r.WithContext(ContextWithSession(r.Context(), sess))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *SecurityHandler) extractSession(r *http.Request) (*session.Session, error) {
	if cookie, err := r.Cookie(s.cookieName); err == nil && cookie.Value != "" {
		return s.auth.store.Get(r.Context(), cookie.Value)
	}
	header := r.Header.Get("Authorization")
	if header != "" {
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			return nil, fmt.Errorf("invalid authorization header")
		}
		return s.auth.store.Get(r.Context(), token)
	}
	return nil, fmt.Errorf("no session found")
}

func SessionFromContext(ctx context.Context) *session.Session {
	sess, _ := ctx.Value(sessionKey).(*session.Session)
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
