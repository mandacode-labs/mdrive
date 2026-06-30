package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

type stateData struct {
	Verifier     string `json:"v"`
	RequestedURI string `json:"ru"`
}

func (s *Service) Authenticate(w http.ResponseWriter, r *http.Request, requestedURI string) {
	verifier := oauth2.GenerateVerifier()
	state := nonce()

	statePayload, err := json.Marshal(stateData{
		Verifier:     verifier,
		RequestedURI: requestedURI,
	})
	if err != nil {
		http.Error(w, "auth: internal error", http.StatusInternalServerError)
		return
	}
	encState, err := encrypt(statePayload, s.encKey)
	if err != nil {
		http.Error(w, "auth: internal error", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, r, "auth_state", encState, 600)
	url := s.oauth2Cfg.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("nonce", nonce()),
	)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "auth: missing code or state", http.StatusBadRequest)
		return
	}

	stateCookie, err := r.Cookie("auth_state")
	if err != nil {
		http.Error(w, "auth: missing state cookie", http.StatusBadRequest)
		return
	}
	rawState, err := decrypt(stateCookie.Value, s.encKey)
	if err != nil {
		http.Error(w, "auth: invalid state cookie", http.StatusBadRequest)
		return
	}
	var sd stateData
	if err := json.Unmarshal(rawState, &sd); err != nil {
		http.Error(w, "auth: invalid state data", http.StatusBadRequest)
		return
	}
	s.setCookie(w, r, "auth_state", "", -1)

	oauth2Token, err := s.oauth2Cfg.Exchange(r.Context(), code, oauth2.VerifierOption(sd.Verifier))
	if err != nil {
		http.Error(w, fmt.Sprintf("auth: token exchange: %v", err), http.StatusBadGateway)
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "auth: missing id_token", http.StatusBadGateway)
		return
	}
	idToken, err := s.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, fmt.Sprintf("auth: verify id_token: %v", err), http.StatusBadGateway)
		return
	}

	userInfo, err := s.provider.UserInfo(r.Context(), oauth2.StaticTokenSource(oauth2Token))
	if err != nil {
		http.Error(w, fmt.Sprintf("auth: userinfo: %v", err), http.StatusBadGateway)
		return
	}
	u, err := s.lookupOrCreateUser(r.Context(), idToken, userInfo)
	if err != nil {
		http.Error(w, "auth: upsert user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	isAdmin := isAdminRole(r.Context(), idToken)

	sess := &sessionData{
		Subject:   idToken.Subject,
		UserID:    u.ID(),
		Provider:  s.providerName,
		IsAdmin:   isAdmin,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}
	if err := s.writeSessionCookie(w, r, sess); err != nil {
		http.Error(w, "auth: write session cookie", http.StatusInternalServerError)
		return
	}

	target := sd.RequestedURI
	if target == "" {
		target = s.postLoginURL
	}
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w, r)

	var metadata struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := s.provider.Claims(&metadata); err != nil || metadata.EndSessionEndpoint == "" {
		target := s.postLogoutURL
		if target == "" {
			target = "/"
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	target := metadata.EndSessionEndpoint + "?post_logout_redirect_uri=" + s.postLogoutURL
	http.Redirect(w, r, target, http.StatusFound)
}

type userInfoClaims struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Service) lookupOrCreateUser(ctx context.Context, idToken *oidc.IDToken, userInfo *oidc.UserInfo) (*user.User, error) {
	sub := idToken.Subject

	u, err := s.users.GetByProviderID(ctx, s.providerName, sub)
	if err == nil && u != nil {
		return u, nil
	}

	if u == nil && s.providerName != "zitadel" {
		u, _ = s.users.GetByProviderID(ctx, "zitadel", sub)
		if u != nil {
			return u, nil
		}
	}

	var claims userInfoClaims
	if err := userInfo.Claims(&claims); err != nil {
		claims.Name = sub
	}
	name := claims.Name
	if name == "" {
		name = sub
	}
	var eptr *string
	if claims.Email != "" {
		eptr = &claims.Email
	}
	return s.users.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       name,
		Email:      eptr,
		Provider:   s.providerName,
		ProviderID: sub,
	})
}
