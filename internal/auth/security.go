package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	zitadelgo "github.com/zitadel/zitadel-go/v3/pkg/authentication"

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
	check := zitadelgo.Middleware[AuthCtx](s.authn).CheckAuthentication()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}
		check(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCtx := zitadelgo.Context[AuthCtx](r.Context())
			if authCtx == nil || authCtx.UserInfo == nil {
				next.ServeHTTP(w, r)
				return
			}
			sub := authCtx.UserInfo.GetSubject()
			if sub == "" {
				next.ServeHTTP(w, r)
				return
			}
			u, err := s.users.GetByProviderID(r.Context(), s.provider, sub)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			sess := &Session{
				ID:        sub,
				UserID:    u.ID(),
				Provider:  s.provider,
				IsAdmin:   isAdminClaim(authCtx),
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(s.sessionTTL),
			}
			r = r.WithContext(ContextWithSession(r.Context(), sess))
			r.Header.Set("Authorization", "Bearer "+u.ID())
			next.ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	})
}

func isAdminClaim(authCtx AuthCtx) bool {
	if authCtx.Tokens == nil || authCtx.Tokens.IDTokenClaims == nil {
		return false
	}
	raw, ok := authCtx.Tokens.IDTokenClaims.Claims[AdminRoleClaim]
	if !ok {
		return false
	}
	roles, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	_, ok = roles[AdminRole]
	return ok
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

var (
	_ http.Handler        = (*Service)(nil)
	_ api.SecurityHandler = (*SecurityHandler)(nil)
)