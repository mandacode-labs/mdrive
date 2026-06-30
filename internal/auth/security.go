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

type SecurityHandler struct {
	auth *Service
}

func NewSecurityHandler(auth *Service) *SecurityHandler {
	return &SecurityHandler{auth: auth}
}

func (s *SecurityHandler) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	if sess := SessionFromContext(ctx); sess != nil {
		return ctx, nil
	}
	return ctx, fmt.Errorf("auth: no session for bearer token")
}

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
		session := &Session{
			ID:        sess.Subject,
			UserID:    u.ID(),
			Provider:  s.providerName,
			IsAdmin:   sess.IsAdmin,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(s.sessionTTL),
		}
		r = r.WithContext(ContextWithSession(r.Context(), session))
		r.Header.Set("Authorization", "Bearer "+u.ID())
		next.ServeHTTP(w, r)
	})
}

func SessionFromContext(ctx context.Context) *Session {
	sess, ok := ctx.Value(sessionKey).(*Session)
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

func IsAdmin(ctx context.Context) bool {
	sess := SessionFromContext(ctx)
	if sess == nil {
		return false
	}
	return sess.IsAdmin
}

func ContextWithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

var _ api.SecurityHandler = (*SecurityHandler)(nil)
