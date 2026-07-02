package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// stateData is the encrypted payload of the "auth_state" cookie.
// It carries the PKCE verifier and the original `state` token
// (to detect CSRF on callback).
type stateData struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
}

// State-cookie failure sentinels. Each carries a distinct message so
// logs and HTTP responses tell the operator exactly which step
// rejected the callback. All four are KindBadRequest because the
// client is at fault in every case (missing/tampered cookie, CSRF
// mismatch). Operators see the difference via the message field.
var (
	errStateMissing  = errorx.New(errorx.KindBadRequest, "auth: missing state cookie")
	errStateDecrypt  = errorx.New(errorx.KindBadRequest, "auth: state cookie decrypt failed")
	errStateCorrupt  = errorx.New(errorx.KindBadRequest, "auth: state cookie corrupt")
	errStateMismatch = errorx.New(errorx.KindBadRequest, "auth: state mismatch")
)

// idTokenClaims captures the standard OIDC ID token claims we
// read for user identity. Keycloak puts name+email in the id_token
// by default, so we don't always need the /userinfo roundtrip.
type idTokenClaims struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// userInfoClaims is the subset of /userinfo we consume.
type userInfoClaims struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Authenticate initiates the OIDC Authorization Code + PKCE flow.
// It mints a state token + PKCE verifier, persists them in an
// encrypted short-lived cookie, and redirects the browser to the
// IdP's authorization endpoint.
func (s *Service) Authenticate(w http.ResponseWriter, r *http.Request) {
	verifier := oauth2.GenerateVerifier()
	state := nonce()

	payload, err := json.Marshal(stateData{
		State:    state,
		Verifier: verifier,
	})
	if err != nil {
		logx.Error(r.Context(), errorx.Wrap(err, "auth: marshal state payload failed"),
			"auth: marshal state payload failed",
			slog.String("path", "/auth/login"),
		)
		http.Error(w, "auth: internal error", http.StatusInternalServerError)
		return
	}
	enc, err := encrypt(payload, s.encKey)
	if err != nil {
		logx.Error(r.Context(), errorx.Wrap(err, "auth: encrypt state failed"),
			"auth: encrypt state failed",
			slog.String("path", "/auth/login"),
		)
		http.Error(w, "auth: internal error", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, r, "auth_state", enc, 600)

	http.Redirect(w, r, s.oauth2Cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// writeError writes a JSON error response using errorx.Kind.Status()
// for the HTTP code. Identical envelope to the rest of the API so
// clients can parse /auth/* errors with the same code path.
func writeError(w http.ResponseWriter, err error) {
	status := statusForError(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// statusForError resolves an error to its HTTP status via errorx.
// Single-source-of-truth lives in errorx.KindOf; this helper is
// kept for backward-compatible callers that need a 500 fallback
// when no errorx.Error is in the chain.
func statusForError(err error) int {
	return errorx.KindOf(err).Status()
}

// Callback exchanges the authorization code for tokens, verifies
// the ID token, upserts the user, and issues the session cookie.
// On success the browser is redirected to the configured
// post-login URL.
func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		err := errorx.New(errorx.KindBadRequest, "auth: missing code or state")
		logx.Error(r.Context(), err, "callback missing code or state",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}

	sd, err := s.consumeStateCookie(w, r, state)
	if err != nil {
		logx.Error(r.Context(), err, "callback state rejected",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}

	token, err := s.oauth2Cfg.Exchange(r.Context(), code, oauth2.VerifierOption(sd.Verifier))
	if err != nil {
		err = errorx.Wrap(err, "auth: token exchange failed")
		logx.Error(r.Context(), err, "token exchange failed",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		err := errorx.New(errorx.KindServiceDegraded, "auth: missing id_token")
		logx.Error(r.Context(), err, "token response missing id_token",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}
	idToken, err := s.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		err = errorx.Wrap(err, "auth: id_token verification failed")
		logx.Error(r.Context(), err, "id_token verification failed",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}

	name, email, err := s.resolveIdentity(r.Context(), idToken, token)
	if err != nil {
		logx.Error(r.Context(), err, "resolve identity failed",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}

	u, err := s.findOrCreateUser(r.Context(), idToken.Subject, name, email)
	if err != nil {
		logx.Error(r.Context(), err, "user upsert failed",
			slog.String("path", "/auth/callback"),
			slog.String("sub", idToken.Subject),
		)
		writeError(w, err)
		return
	}

	sess := &sessionData{
		Subject:   idToken.Subject,
		UserID:    u.ID(),
		Provider:  s.providerName,
		IsAdmin:   isAdminRole(idToken),
		IDToken:   rawIDToken,
		Name:      name,
		Email:     email,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	if err := s.writeSessionCookie(w, r, sess); err != nil {
		logx.Error(r.Context(), err, "write session cookie failed",
			slog.String("path", "/auth/callback"),
		)
		writeError(w, err)
		return
	}

	target := s.postLoginURL
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// consumeStateCookie reads, decrypts, and clears the state cookie,
// then verifies the `state` query parameter matches the stored one.
// This is the CSRF guard per RFC 6749 §10.12. The returned error is
// always an errorx.KindBadRequest sentinel so the response handler
// can map it to 400 and the operator can read the exact failure
// reason from the message field.
func (s *Service) consumeStateCookie(w http.ResponseWriter, r *http.Request, state string) (*stateData, error) {
	c, err := r.Cookie("auth_state")
	if err != nil {
		return nil, errorx.Wrap(errStateMissing, "auth: consume state cookie missing")
	}
	s.setCookie(w, r, "auth_state", "", -1)
	raw, err := decrypt(c.Value, s.encKey)
	if err != nil {
		return nil, errorx.Wrap(errStateDecrypt, "auth: consume state cookie decrypt failed")
	}
	var sd stateData
	if err := json.Unmarshal(raw, &sd); err != nil {
		return nil, errorx.Wrap(errStateCorrupt, "auth: consume state cookie corrupt")
	}
	if sd.State == "" || sd.State != state {
		return nil, errorx.Wrap(errStateMismatch, fmt.Sprintf("auth: consume state cookie mismatch (have=%q want=%q)", sd.State, state))
	}
	return &sd, nil
}

// Logout clears the session cookie and redirects to the IdP's
// RP-initiated logout endpoint with `id_token_hint` and
// `client_id` so Keycloak terminates the SSO session, not just
// ours. Per OpenID Connect RP-Initiated Logout 1.0 §2.1.
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	sess, _ := s.readSessionCookie(r)
	s.clearSessionCookie(w, r)

	target := s.postLogoutURL
	if target == "" {
		target = "/"
	}

	var metadata struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := s.provider.Claims(&metadata); err != nil || metadata.EndSessionEndpoint == "" {
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	q := url.Values{
		"post_logout_redirect_uri": {target},
		"client_id":                {s.oauth2Cfg.ClientID},
	}
	if sess != nil && sess.IDToken != "" {
		q.Set("id_token_hint", sess.IDToken)
	}
	http.Redirect(w, r, metadata.EndSessionEndpoint+"?"+q.Encode(), http.StatusFound)
}

// resolveIdentity reads name+email from the id_token claims first,
// falling back to /userinfo if either is empty. /userinfo is skipped
// entirely when the id_token already carries both.
func (s *Service) resolveIdentity(ctx context.Context, idToken *oidc.IDToken, token *oauth2.Token) (string, string, error) {
	var claims idTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return "", "", errorx.Wrap(err, "auth: decode id_token claims")
	}
	if claims.Name != "" && claims.Email != "" {
		return claims.Name, claims.Email, nil
	}

	userInfo, err := s.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		if claims.Name != "" || claims.Email != "" {
			return claims.Name, claims.Email, nil
		}
		return "", "", errorx.Wrap(err, "auth: userinfo")
	}
	var ui userInfoClaims
	if err := userInfo.Claims(&ui); err != nil {
		if claims.Name != "" || claims.Email != "" {
			return claims.Name, claims.Email, nil
		}
		return "", "", errorx.Wrap(err, "auth: decode userinfo claims")
	}
	if claims.Name == "" {
		claims.Name = ui.Name
	}
	if claims.Email == "" {
		claims.Email = ui.Email
	}
	return claims.Name, claims.Email, nil
}

// findOrCreateUser looks up the OIDC subject, falling back to the
// legacy "zitadel" provider string for users created before the
// Keycloak migration. New users are tagged with the configured
// provider name.
func (s *Service) findOrCreateUser(ctx context.Context, sub, name, email string) (*user.User, error) {
	if u, _ := s.users.GetByProviderID(ctx, s.providerName, sub); u != nil {
		return u, nil
	}
	if s.providerName != "zitadel" {
		if u, _ := s.users.GetByProviderID(ctx, "zitadel", sub); u != nil {
			return u, nil
		}
	}

	displayName := name
	if displayName == "" {
		displayName = sub
	}
	var eptr *string
	if email != "" {
		eptr = &email
	}
	return s.users.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       displayName,
		Email:      eptr,
		Provider:   s.providerName,
		ProviderID: sub,
	})
}
