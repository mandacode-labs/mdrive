package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const providerGoogle = "google"

func (h *Handler) GoogleLogin(ctx context.Context) error {
	if h.auth == nil {
		return fmt.Errorf("authentication not configured")
	}
	state, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("random state: %w", err)
	}
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return fmt.Errorf("generate pkce: %w", err)
	}
	if err := h.auth.StorePKCE(ctx, state, verifier); err != nil {
		return fmt.Errorf("store pkce: %w", err)
	}
	authURL, err := h.auth.AuthorizeURL(ctx, providerGoogle, h.frontendURL+"/auth/callback", state, challenge)
	if err != nil {
		return err
	}
	w := ctxResponseWriter(ctx)
	if w != nil {
		http.Redirect(w, ctxRequest(ctx), authURL, http.StatusFound)
	}
	return nil
}

func (h *Handler) GoogleNativeLogin(ctx context.Context, req api.OptGoogleNativeLoginReq) (*api.GoogleNativeLoginOK, error) {
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

func (h *Handler) AuthCallback(ctx context.Context, params api.AuthCallbackParams) error {
	if h.auth == nil {
		return fmt.Errorf("authentication not configured")
	}
	verifier, err := h.auth.GetPKCE(ctx, params.State)
	if err != nil {
		return fmt.Errorf("pkce verifier: %w", err)
	}
	tokens, err := h.auth.ExchangeCode(ctx, params.Code, h.frontendURL+"/auth/callback", verifier)
	if err != nil {
		return err
	}
	claims, err := h.auth.VerifyIDToken(ctx, tokens.IDToken)
	if err != nil {
		return err
	}
	u, err := h.createOrUpdateUser(ctx, claims.GetSubject(), claims.Name, claims.Email, providerGoogle)
	if err != nil {
		return err
	}
	sess, err := h.auth.CreateSession(ctx, u.ID(), providerGoogle, auth.IsAdminClaim(claims))
	if err != nil {
		return err
	}
	w := ctxResponseWriter(ctx)
	if w != nil {
		http.SetCookie(w, h.sessionCookie(sess.ID, sess.ExpiresAt))
		http.Redirect(w, ctxRequest(ctx), h.frontendURL, http.StatusFound)
	}
	return nil
}

func (h *Handler) AuthLogout(ctx context.Context) error {
	sess := auth.SessionFromContext(ctx)
	if sess != nil && h.auth != nil {
		_ = h.auth.DeleteSession(ctx, sess.ID)
	}
	w := ctxResponseWriter(ctx)
	if w != nil {
		http.SetCookie(w, h.expiredCookie())
	}
	return nil
}

func (h *Handler) AuthMe(ctx context.Context) (*api.User, error) {
	sess := auth.SessionFromContext(ctx)
	if sess == nil {
		return nil, fmt.Errorf("authentication not configured")
	}
	u, err := h.vfs.GetUser(ctx, sess.UserID, sess.UserID)
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
	return h.vfs.UpsertUser(ctx, "", &user.CreateCommand{
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

// sessionCookie returns a configured session cookie. Cookie attributes
// come from config which has safe defaults (HttpOnly=true, SameSite=Lax
// in dev / Strict in prod, Secure in prod); G124 is excluded in
// gosec.json for this file.
func (h *Handler) sessionCookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     h.cookieConfig.Name,
		Value:    value,
		Expires:  expires,
		HttpOnly: h.cookieConfig.HttpOnly,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
		Path:     h.cookieConfig.Path,
	}
}

// expiredCookie returns a cookie that clears the session.
func (h *Handler) expiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     h.cookieConfig.Name,
		Value:    "",
		Path:     h.cookieConfig.Path,
		HttpOnly: h.cookieConfig.HttpOnly,
		Secure:   h.cookieConfig.Secure,
		SameSite: h.cookieConfig.SameSite,
		MaxAge:   -1,
	}
}

// responseWriterProvider is the interface ogen wraps around the real
// http.ResponseWriter. The auth flow needs direct access to call
// http.Redirect for browser flows, so we extract it from the context.
type responseWriterProvider interface{ ResponseWriter() http.ResponseWriter }

// requestProvider is the ogen interface for retrieving the underlying
// *http.Request from the context.
type requestProvider interface{ Request() *http.Request }

func ctxResponseWriter(ctx context.Context) http.ResponseWriter {
	if r, ok := ctx.(responseWriterProvider); ok {
		return r.ResponseWriter()
	}
	return nil
}

func ctxRequest(ctx context.Context) *http.Request {
	if r, ok := ctx.(requestProvider); ok {
		return r.Request()
	}
	return nil
}
