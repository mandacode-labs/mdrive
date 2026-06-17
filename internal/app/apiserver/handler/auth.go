package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const providerGoogle = "google"

func (h *Handler) GoogleLogin(ctx context.Context) error {
	if h.auth == nil {
		return errNotConfigured()
	}
	state := mustRandomHex(32)
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
		return nil, errNotConfigured()
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
	sess, err := h.auth.CreateSession(ctx, u.ID(), providerGoogle)
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
		return errNotConfigured()
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
	sess, err := h.auth.CreateSession(ctx, u.ID(), providerGoogle)
	if err != nil {
		return err
	}
	w := ctxResponseWriter(ctx)
	if w != nil {
		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    sess.ID,
			Expires:  sess.ExpiresAt,
			HttpOnly: true,
			Secure:   h.secureCookie,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
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
		http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: "", MaxAge: -1})
	}
	return nil
}

func (h *Handler) AuthMe(ctx context.Context) (*api.User, error) {
	sess := auth.SessionFromContext(ctx)
	if sess == nil {
		return nil, errNotConfigured()
	}
	u, err := h.vfs.GetUser(ctx, sess.UserID)
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
	return h.vfs.UpsertUser(ctx, &user.CreateCommand{
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

func mustRandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func errNotConfigured() error {
	return fmt.Errorf("authentication not configured")
}

func ctxResponseWriter(ctx context.Context) http.ResponseWriter {
	type rw interface{ ResponseWriter() http.ResponseWriter }
	if r, ok := ctx.(rw); ok {
		return r.ResponseWriter()
	}
	return nil
}

func ctxRequest(ctx context.Context) *http.Request {
	type rq interface{ Request() *http.Request }
	if r, ok := ctx.(rq); ok {
		return r.Request()
	}
	return nil
}
