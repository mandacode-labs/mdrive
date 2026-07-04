package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type contextKey string

const sessionKey contextKey = "session"

// HandleCookieAuth satisfies ogen's api.SecurityHandler contract.
// t.APIKey is the raw AES-GCM-encrypted cookie payload. On any
// failure it returns KindUnauthenticated so the response maps to
// 401, not the 500 that fmt.Errorf used to produce.
//
// /health is anonymous by OpenAPI spec (`security: []`); ogen does
// not invoke this handler for that operation.
func (s *Service) HandleCookieAuth(ctx context.Context, _ api.OperationName, t api.CookieAuth) (context.Context, error) {
	if t.APIKey == "" {
		return ctx, errorx.New(errorx.KindUnauthenticated, "auth: no session cookie")
	}
	sess, err := s.readSessionCookieFromValue(t.APIKey)
	if err != nil || sess == nil {
		return ctx, errorx.New(errorx.KindUnauthenticated, "auth: invalid session cookie")
	}
	u, err := s.users.GetByProviderID(ctx, s.providerName, sess.Subject)
	if err != nil || u == nil {
		return ctx, errorx.New(errorx.KindUnauthenticated, "auth: session user not found")
	}
	return ContextWithSession(ctx, &Session{
		ID:        sess.Subject,
		UserID:    u.ID(),
		Provider:  s.providerName,
		IsAdmin:   sess.IsAdmin,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}), nil
}

// readSessionCookieFromValue decrypts a raw cookie value. It is the
// value-side counterpart to readSessionCookie, which reads from
// *http.Request. HandleCookieAuth gets the raw value from ogen.
func (s *Service) readSessionCookieFromValue(value string) (*sessionData, error) {
	raw, err := decrypt(value, s.encKey)
	if err != nil {
		return nil, err
	}
	var data sessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.IsExpired() {
		return nil, errorx.New(errorx.KindBadRequest, "session: expired")
	}
	return &data, nil
}

// SessionFromContext returns the Session attached by HandleCookieAuth,
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

var _ api.SecurityHandler = (*Service)(nil)
