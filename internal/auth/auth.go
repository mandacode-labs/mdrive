package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

type UserUpserter interface {
	UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*user.User, error)
}

type Config struct {
	Issuer         string
	ClientID       string
	ClientSecret   string
	RedirectURI    string
	PostLoginURL   string
	PostLogoutURL  string
	CookieName     string
	CookieDomain   string
	CookieSameSite http.SameSite
	EncryptionKey  string
	SessionTTL     time.Duration
	Scopes         []string
	Provider       string
}

type Service struct {
	provider       *oidc.Provider
	verifier       *oidc.IDTokenVerifier
	oauth2Cfg      oauth2.Config
	encKey         []byte
	users          UserUpserter
	providerName   string
	cookieName     string
	cookieDomain   string
	cookieSameSite http.SameSite
	postLoginURL   string
	postLogoutURL  string
	sessionTTL     time.Duration
	scopes         []string
}

func New(ctx context.Context, cfg Config, users UserUpserter) (*Service, error) {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "mdrive_session"
	}
	if cfg.CookieSameSite == 0 {
		cfg.CookieSameSite = http.SameSiteLaxMode
	}

	p, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discover provider: %w", err)
	}

	svc := &Service{
		provider: p,
		verifier: p.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2Cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     p.Endpoint(),
			RedirectURL:  cfg.RedirectURI,
			Scopes:       cfg.Scopes,
		},
		encKey:         []byte(cfg.EncryptionKey),
		users:          users,
		providerName:   cfg.Provider,
		cookieName:     cfg.CookieName,
		cookieDomain:   cfg.CookieDomain,
		cookieSameSite: cfg.CookieSameSite,
		postLoginURL:   cfg.PostLoginURL,
		postLogoutURL:  cfg.PostLogoutURL,
		sessionTTL:     cfg.SessionTTL,
		scopes:         cfg.Scopes,
	}
	return svc, nil
}

func (s *Service) PostLoginURL() string {
	return s.postLoginURL
}

func (s *Service) PostLogoutURL() string {
	return s.postLogoutURL
}
