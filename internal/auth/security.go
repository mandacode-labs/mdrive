package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"

	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type contextKey string

const sessionKey contextKey = "session"

type Config struct {
	Issuer       string
	ClientID     string
	SessionStore session.Store
	SessionTTL   time.Duration
	FrontendURL  string
}

type Authenticator struct {
	cfg        Config
	store      session.Store
	verifier   *rp.IDTokenVerifier
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewAuthenticator(ctx context.Context, cfg Config) (*Authenticator, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	dc, err := client.Discover(ctx, cfg.Issuer, httpClient)
	if err != nil {
		return nil, fmt.Errorf("auth: discover: %w", err)
	}

	keySet := rp.NewRemoteKeySet(httpClient, dc.JwksURI)

	verifier := rp.NewIDTokenVerifier(cfg.Issuer, cfg.ClientID, keySet,
		rp.WithIssuedAtMaxAge(1*time.Hour),
	)

	return &Authenticator{
		cfg:        cfg,
		store:      cfg.SessionStore,
		verifier:   verifier,
		httpClient: httpClient,
	}, nil
}

func (a *Authenticator) Discovery(ctx context.Context) (*oidc.DiscoveryConfiguration, error) {
	return client.Discover(ctx, a.cfg.Issuer, a.httpClient)
}

// ExchangeJWT exchanges an external JWT (Google id_token) for Zitadel tokens.
func (a *Authenticator) ExchangeJWT(ctx context.Context, assertion string) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	dc, err := client.Discover(ctx, a.cfg.Issuer, a.httpClient)
	if err != nil {
		return nil, err
	}
	request := oidc.NewJWTProfileGrantRequest(assertion, "openid", "profile", "email")
	caller := &tokenEndpointCaller{dc: dc, http: a.httpClient}

	token, err := client.CallTokenEndpoint(ctx, request, caller)
	if err != nil {
		return nil, fmt.Errorf("auth: jwt profile: %w", err)
	}
	idToken, _ := token.Extra("id_token").(string)
	claims, _ := a.verifyIDToken(ctx, idToken)
	return &oidc.Tokens[*oidc.IDTokenClaims]{
		Token:        token,
		IDTokenClaims: claims,
		IDToken:      idToken,
	}, nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (a *Authenticator) ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	dc, err := client.Discover(ctx, a.cfg.Issuer, a.httpClient)
	if err != nil {
		return nil, err
	}
	request := &oidc.AccessTokenRequest{
		Code:         code,
		RedirectURI:  redirectURI,
		ClientID:     a.cfg.ClientID,
		CodeVerifier: codeVerifier,
	}
	caller := &tokenEndpointCaller{dc: dc, http: a.httpClient}

	token, err := client.CallTokenEndpoint(ctx, request, caller)
	if err != nil {
		return nil, fmt.Errorf("auth: code exchange: %w", err)
	}
	idToken, _ := token.Extra("id_token").(string)
	claims, _ := a.verifyIDToken(ctx, idToken)
	return &oidc.Tokens[*oidc.IDTokenClaims]{
		Token:        token,
		IDTokenClaims: claims,
		IDToken:      idToken,
	}, nil
}

// AuthorizeURL builds the authorization URL for the given provider.
func (a *Authenticator) AuthorizeURL(ctx context.Context, provider, redirectURI, state, codeChallenge string) (string, error) {
	dc, err := client.Discover(ctx, a.cfg.Issuer, a.httpClient)
	if err != nil {
		return "", err
	}
	q := make(url.Values)
	q.Set("client_id", a.cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge_method", "S256")
	q.Set("code_challenge", codeChallenge)
	if provider != "" {
		q.Set("idp_id", provider)
	}
	return dc.AuthorizationEndpoint + "?" + q.Encode(), nil
}

// CreateSession creates a new authenticated session.
func (a *Authenticator) CreateSession(ctx context.Context, userID, provider string) (*session.Session, error) {
	sess := session.New(a.cfg.SessionTTL)
	sess.UserID = userID
	sess.Provider = provider
	if err := a.store.Create(ctx, sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// DeleteSession removes a session.
func (a *Authenticator) DeleteSession(ctx context.Context, id string) error {
	return a.store.Delete(ctx, id)
}

// VerifyIDToken validates an id_token and returns the claims.
func (a *Authenticator) VerifyIDToken(ctx context.Context, raw string) (*oidc.IDTokenClaims, error) {
	return a.verifyIDToken(ctx, raw)
}

func (a *Authenticator) verifyIDToken(ctx context.Context, raw string) (*oidc.IDTokenClaims, error) {
	if raw == "" {
		return nil, fmt.Errorf("auth: empty id_token")
	}
	claims, err := rp.VerifyIDToken[*oidc.IDTokenClaims](ctx, raw, a.verifier)
	if err != nil {
		return nil, fmt.Errorf("auth: verify id_token: %w", err)
	}
	return claims, nil
}

type SecurityHandler struct {
	auth       *Authenticator
	cookieName string
}

func NewSecurityHandler(auth *Authenticator) *SecurityHandler {
	return &SecurityHandler{auth: auth, cookieName: "mdrive_sid"}
}

func (s *SecurityHandler) HandleBearerAuth(ctx context.Context, _ api.OperationName, t api.BearerAuth) (context.Context, error) {
	if SessionFromContext(ctx) != nil {
		return ctx, nil
	}
	sess, err := s.auth.store.Get(ctx, t.Token)
	if err != nil {
		return ctx, fmt.Errorf("bearer session: %w", err)
	}
	return ContextWithSession(ctx, sess), nil
}

func (s *SecurityHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := s.extractSession(r)
		if err == nil {
			r = r.WithContext(ContextWithSession(r.Context(), sess))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *SecurityHandler) extractSession(r *http.Request) (*session.Session, error) {
	if cookie, err := r.Cookie(s.cookieName); err == nil && cookie.Value != "" {
		return s.auth.store.Get(r.Context(), cookie.Value)
	}
	header := r.Header.Get("Authorization")
	if header != "" {
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			return nil, fmt.Errorf("invalid authorization header")
		}
		return s.auth.store.Get(r.Context(), token)
	}
	return nil, fmt.Errorf("no session found")
}

func SessionFromContext(ctx context.Context) *session.Session {
	sess, _ := ctx.Value(sessionKey).(*session.Session)
	return sess
}

func UserIDFromContext(ctx context.Context) string {
	sess := SessionFromContext(ctx)
	if sess == nil {
		return ""
	}
	return sess.UserID
}

func ContextWithSession(ctx context.Context, s *session.Session) context.Context {
	return context.WithValue(ctx, sessionKey, s)
}

type tokenEndpointCaller struct {
	dc   *oidc.DiscoveryConfiguration
	http *http.Client
}

func (t *tokenEndpointCaller) TokenEndpoint() string   { return t.dc.TokenEndpoint }
func (t *tokenEndpointCaller) HttpClient() *http.Client { return t.http }

var _ api.SecurityHandler = (*SecurityHandler)(nil)
var _ = oauth2.Token{}
