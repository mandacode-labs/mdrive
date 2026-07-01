package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type contextKey string

const sessionKey contextKey = "session"

// SecurityHandler satisfies ogen's api.SecurityHandler contract.
// It is the bearer-token path: when Middleware has already
// attached a Session to the context, validation succeeds;
// otherwise we return errorx.KindUnauthenticated so the response
// maps to 401, not the raw 500 that fmt.Errorf produced before.
type SecurityHandler struct {
	auth *Service
}

func NewSecurityHandler(auth *Service) *SecurityHandler {
	return &SecurityHandler{auth: auth}
}

// HandleBearerAuth returns errorx.KindUnauthenticated when no
// session is attached to the request context. This is the ogen
// SecurityHandler entry point; an error here propagates through
// WithErrorHandler -> NewError -> errorx -> 401 response.
//
// Returning a raw fmt.Errorf used to fall through to the 500
// fallback in NewError, which is why /auth/me (and every other
// authenticated endpoint) returned 500 when the cookie bridge
// failed silently.
func (s *SecurityHandler) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	if SessionFromContext(ctx) != nil {
		return ctx, nil
	}
	return ctx, errorx.New(errorx.KindUnauthenticated, "auth: no session for bearer token")
}

// Middleware bridges cookie-based OIDC sessions to ogen's
// bearer-auth contract. If the request already carries an
// Authorization header, it is passed through unchanged. Otherwise
// the session cookie is read, the user is looked up, and a
// synthesized "Bearer <userID>" header plus a context-bound
// Session are attached for downstream handlers.
//
// When the cookie is missing, expired, or does not match a known
// user, Middleware responds 401 directly via writeAuthError
// instead of letting the request reach ogen unauthenticated
// (which used to surface as 500 from NewError's fallback path).
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			next.ServeHTTP(w, r)
			return
		}
		sess, err := s.readSessionCookie(r)
		if err != nil || sess == nil {
			writeAuthError(w, errorx.New(errorx.KindUnauthenticated, "auth: missing or invalid session cookie"))
			return
		}
		u, err := s.users.GetByProviderID(r.Context(), s.providerName, sess.Subject)
		if err != nil || u == nil {
			writeAuthError(w, errorx.New(errorx.KindUnauthenticated, "auth: session user not found"))
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

// writeAuthError writes a JSON error response using errorx's HTTP
// status mapping. Used by Middleware to short-circuit unauthenticated
// requests with a real 401 instead of letting them reach ogen and
// surface as a generic 500.
func writeAuthError(w http.ResponseWriter, err error) {
	status := errorx.KindServiceDegraded.Status() // fallback
	var de errorx.Error
	if errors.As(err, &de) {
		status = de.Kind().Status()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
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