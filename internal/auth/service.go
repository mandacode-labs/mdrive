package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/mandacode-labs/mdrive/internal/auth/session"
)

type Config struct {
	Issuer       string
	ClientID     string
	SessionStore session.Store
	SessionTTL   time.Duration
}

// Service manages OIDC flows and session lifecycle with Zitadel.
type Service struct {
	config     Config
	store      session.Store
	verifier   *rp.IDTokenVerifier
	httpClient *http.Client
}

func NewService(ctx context.Context, config Config) (*Service, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	dc, err := client.Discover(ctx, config.Issuer, httpClient)
	if err != nil {
		return nil, fmt.Errorf("auth: discover: %w", err)
	}

	keySet := rp.NewRemoteKeySet(httpClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(config.Issuer, config.ClientID, keySet,
		rp.WithIssuedAtMaxAge(1*time.Hour),
	)

	return &Service{
		config:      config,
		store:      config.SessionStore,
		verifier:   verifier,
		httpClient: httpClient,
	}, nil
}

func (s *Service) Discovery(ctx context.Context) (*oidc.DiscoveryConfiguration, error) {
	return client.Discover(ctx, s.config.Issuer, s.httpClient)
}

func (s *Service) ExchangeJWT(ctx context.Context, assertion string) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	dc, err := client.Discover(ctx, s.config.Issuer, s.httpClient)
	if err != nil {
		return nil, err
	}
	request := oidc.NewJWTProfileGrantRequest(assertion, scopeOpenID, scopeProfile, scopeEmail)
	caller := &tokenEndpointCaller{dc: dc, http: s.httpClient}

	token, err := client.CallTokenEndpoint(ctx, request, caller)
	if err != nil {
		return nil, fmt.Errorf("auth: jwt profile: %w", err)
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("auth: jwt profile: id_token not in response")
	}
	claims, err := s.verifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("auth: jwt profile: %w", err)
	}
	return &oidc.Tokens[*oidc.IDTokenClaims]{
		Token:         token,
		IDTokenClaims: claims,
		IDToken:       idToken,
	}, nil
}

func (s *Service) ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	dc, err := client.Discover(ctx, s.config.Issuer, s.httpClient)
	if err != nil {
		return nil, err
	}
	request := &oidc.AccessTokenRequest{
		Code:         code,
		RedirectURI:  redirectURI,
		ClientID:     s.config.ClientID,
		CodeVerifier: codeVerifier,
	}
	caller := &tokenEndpointCaller{dc: dc, http: s.httpClient}

	token, err := client.CallTokenEndpoint(ctx, request, caller)
	if err != nil {
		return nil, fmt.Errorf("auth: code exchange: %w", err)
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return nil, fmt.Errorf("auth: code exchange: id_token not in response")
	}
	claims, err := s.verifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("auth: code exchange: %w", err)
	}
	return &oidc.Tokens[*oidc.IDTokenClaims]{
		Token:         token,
		IDTokenClaims: claims,
		IDToken:       idToken,
	}, nil
}

func (s *Service) AuthorizeURL(ctx context.Context, provider, redirectURI, state, codeChallenge string) (string, error) {
	dc, err := client.Discover(ctx, s.config.Issuer, s.httpClient)
	if err != nil {
		return "", err
	}
	q := make(url.Values)
	q.Set("client_id", s.config.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", DefaultOIDCScopes)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge_method", "S256")
	q.Set("code_challenge", codeChallenge)
	if provider != "" {
		q.Set("idp_id", provider)
	}
	return dc.AuthorizationEndpoint + "?" + q.Encode(), nil
}

func (s *Service) CreateSession(ctx context.Context, userID, provider string, isAdmin bool) (*session.Session, error) {
	sess := session.New(s.config.SessionTTL)
	sess.UserID = userID
	sess.Provider = provider
	sess.IsAdmin = isAdmin
	if err := s.store.Create(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// IsAdminClaim returns true if the claims contain the configured admin role.
func IsAdminClaim(claims *oidc.IDTokenClaims) bool {
	if claims == nil {
		return false
	}
	raw, ok := claims.Claims[AdminRoleClaim]
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

func (s *Service) DeleteSession(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}

func (s *Service) StorePKCE(ctx context.Context, state, verifier string) error {
	sess := session.New(5 * time.Minute)
	sess.ID = PKCEPrefix + state
	sess.UserID = verifier
	return s.store.Create(ctx, sess)
}

func (s *Service) GetPKCE(ctx context.Context, state string) (string, error) {
	sess, err := s.store.Get(ctx, PKCEPrefix+state)
	if err != nil {
		return "", err
	}
	// Best-effort cleanup; key has short TTL anyway.
	_ = s.store.Delete(ctx, PKCEPrefix+state)
	return sess.UserID, nil
}

func (s *Service) VerifyIDToken(ctx context.Context, raw string) (*oidc.IDTokenClaims, error) {
	return s.verifyIDToken(ctx, raw)
}

func (s *Service) verifyIDToken(ctx context.Context, raw string) (*oidc.IDTokenClaims, error) {
	if raw == "" {
		return nil, fmt.Errorf("auth: empty id_token")
	}
	claims, err := rp.VerifyIDToken[*oidc.IDTokenClaims](ctx, raw, s.verifier)
	if err != nil {
		return nil, fmt.Errorf("auth: verify id_token: %w", err)
	}
	return claims, nil
}

type tokenEndpointCaller struct {
	dc   *oidc.DiscoveryConfiguration
	http *http.Client
}

func (t *tokenEndpointCaller) TokenEndpoint() string    { return t.dc.TokenEndpoint }
func (t *tokenEndpointCaller) HttpClient() *http.Client { return t.http }
