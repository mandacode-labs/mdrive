package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const providerGoogle = "google"

func (h *Handler) GoogleLogin(ctx context.Context) (*api.GoogleLoginFound, error) {
	if h.auth == nil {
		return nil, fmt.Errorf("authentication not configured")
	}
	state, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("random state: %w", err)
	}
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("generate pkce: %w", err)
	}
	if err := h.auth.StorePKCE(ctx, state, verifier); err != nil {
		return nil, fmt.Errorf("store pkce: %w", err)
	}
	authURL, err := h.auth.AuthorizeURL(ctx, providerGoogle, h.frontendURL+"/auth/callback", state, challenge)
	if err != nil {
		return nil, err
	}
	return &api.GoogleLoginFound{Location: authURL}, nil
}

func (h *Handler) GoogleNativeLogin(ctx context.Context, req api.OptGoogleNativeLoginReq) (api.GoogleNativeLoginRes, error) {
	if h.auth == nil {
		return nil, fmt.Errorf("authentication not configured")
	}
	r := req.Value
	tokens, err := h.auth.ExchangeJWT(ctx, r.IdToken)
	if err != nil {
		return nil, err
	}
	claims, err := h.auth.VerifyIDToken(ctx, tokens.IDToken)
	if err != nil {
		return nil, err
	}
	u, err := h.createOrUpdateUser(ctx, claims.GetSubject(), claims.Name, claims.Email, providerGoogle)
	if err != nil {
		return nil, err
	}
	sess, err := h.auth.CreateSession(ctx, u.ID(), providerGoogle, auth.IsAdminClaim(claims))
	if err != nil {
		return nil, err
	}
	return &api.GoogleNativeLoginOK{
		Token: api.OptString{Value: sess.ID, Set: true},
		User:  api.OptUser{Value: *userToAPI(u), Set: true},
	}, nil
}

func (h *Handler) AuthCallback(ctx context.Context, params api.AuthCallbackParams) (api.AuthCallbackRes, error) {
	if h.auth == nil {
		return nil, fmt.Errorf("authentication not configured")
	}
	verifier, err := h.auth.GetPKCE(ctx, params.State)
	if err != nil {
		return nil, fmt.Errorf("pkce verifier: %w", err)
	}
	tokens, err := h.auth.ExchangeCode(ctx, params.Code, h.frontendURL+"/auth/callback", verifier)
	if err != nil {
		return nil, err
	}
	claims, err := h.auth.VerifyIDToken(ctx, tokens.IDToken)
	if err != nil {
		return nil, err
	}
	u, err := h.createOrUpdateUser(ctx, claims.GetSubject(), claims.Name, claims.Email, providerGoogle)
	if err != nil {
		return nil, err
	}
	sess, err := h.auth.CreateSession(ctx, u.ID(), providerGoogle, auth.IsAdminClaim(claims))
	if err != nil {
		return nil, err
	}
	return &api.AuthCallbackFound{
		Location:  h.frontendURL,
		SetCookie: h.sessionCookie(sess.ID, sess.ExpiresAt).String(),
	}, nil
}

func (h *Handler) AuthLogout(ctx context.Context) (api.AuthLogoutRes, error) {
	sess := auth.SessionFromContext(ctx)
	if sess != nil {
		if h.auth == nil {
			return nil, errors.New("auth: logout unavailable: session backend not configured")
		}
		if err := h.auth.DeleteSession(ctx, sess.ID); err != nil {
			return nil, fmt.Errorf("auth: logout: delete session: %w", err)
		}
	}
	return &api.AuthLogoutNoContent{SetCookie: h.expiredCookie().String()}, nil
}

// ErrUnauthenticated is returned by AuthMe when no session is in
// the request context. It maps to HTTP 401 in FromError; using a
// local sentinel avoids leaking ogen-internal errors from this
// package.
var ErrUnauthenticated = errors.New("auth: not authenticated")

func (h *Handler) AuthMe(ctx context.Context) (api.AuthMeRes, error) {
	sess := auth.SessionFromContext(ctx)
	if sess == nil {
		return nil, ErrUnauthenticated
	}
	u, err := h.users.GetByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	return userToAPI(u), nil
}

func (h *Handler) createOrUpdateUser(ctx context.Context, sub, name, email, provider string) (*user.User, error) {
	var eptr *string
	if email != "" {
		eptr = &email
	}
	return h.users.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       name,
		Email:      eptr,
		Provider:   provider,
		ProviderID: sub,
	})
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return
}

// randomHex returns a hex-encoded string of n random bytes. The error
// is returned to the caller rather than panicked: in practice
// crypto/rand only fails in catastrophic situations, but panic in a
// request handler is the wrong failure mode.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// cookieBase returns the shared cookie attributes (Name, Path,
// HttpOnly, Secure, SameSite). Callers set Value/Expires/MaxAge
// per use.
func (h *Handler) cookieBase() http.Cookie {
	return http.Cookie{
		Name:     h.cookieConfig.Name,
		Path:     h.cookieConfig.Path,
		HttpOnly: h.cookieConfig.HttpOnly,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSiteMode(),
	}
}

// sessionCookie returns a session cookie carrying value with
// the given expiry.
func (h *Handler) sessionCookie(value string, expires time.Time) *http.Cookie {
	c := h.cookieBase()
	c.Value = value
	c.Expires = expires
	return &c
}

// expiredCookie returns a cookie that instructs the browser to
// delete the session.
func (h *Handler) expiredCookie() *http.Cookie {
	c := h.cookieBase()
	c.Value = ""
	c.MaxAge = -1
	return &c
}
