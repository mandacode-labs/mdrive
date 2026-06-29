// Package auth wraps zitadel-go's OIDC authentication middleware
// and bridges its cookie-based sessions into the rest of the app.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	oidc "github.com/zitadel/oidc/v3/pkg/oidc"
	zitadelgo "github.com/zitadel/zitadel-go/v3/pkg/authentication"
	zitadeloidc "github.com/zitadel/zitadel-go/v3/pkg/authentication/oidc"
	zitadelcfg "github.com/zitadel/zitadel-go/v3/pkg/zitadel"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

type UserUpserter interface {
	UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*user.User, error)
}

type Config struct {
	Issuer        string
	ClientID      string
	RedirectURI   string
	PostLogoutURL string
	CookieName    string
	EncryptionKey string
	SessionTTL    time.Duration
	Scopes        []string
	Provider      string
}

type AuthCtx = *zitadeloidc.UserInfoContext[*oidc.IDTokenClaims, *oidc.UserInfo]

type Service struct {
	authn         *zitadelgo.Authenticator[AuthCtx]
	users         UserUpserter
	provider      string
	cookieName    string
	postLogoutURL string
	sessionTTL    time.Duration
}

func New(ctx context.Context, cfg Config, users UserUpserter) (*Service, error) {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}
	}
	if cfg.CookieName == "" {
		cfg.CookieName = "mdrive_session"
	}

	z := zitadelcfg.New(cfg.Issuer)
	svc := &Service{
		users:         users,
		provider:      cfg.Provider,
		cookieName:    cfg.CookieName,
		postLogoutURL: cfg.PostLogoutURL,
		sessionTTL:    cfg.SessionTTL,
	}

	var err error
	svc.authn, err = zitadelgo.New(
		ctx, z, cfg.EncryptionKey,
		zitadeloidc.DefaultAuthentication(cfg.ClientID, cfg.RedirectURI, cfg.EncryptionKey, cfg.Scopes...),
		zitadelgo.WithCookieSession[AuthCtx](),
		zitadelgo.WithSessionCookieName[AuthCtx](cfg.CookieName),
		zitadelgo.WithPostLogoutRedirectURI[AuthCtx](cfg.PostLogoutURL),
		zitadelgo.WithOnAuthenticated[AuthCtx](svc.upsertUser),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: init: %w", err)
	}
	return svc, nil
}

func (s *Service) upsertUser(ctx context.Context, authCtx AuthCtx) error {
	info := authCtx.UserInfo
	if info == nil {
		return fmt.Errorf("auth: missing user info in callback")
	}
	name := info.Name
	email := info.Email
	if name == "" {
		name = email
	}
	if name == "" {
		name = info.GetSubject()
	}
	var eptr *string
	if email != "" {
		eptr = &email
	}
	_, err := s.users.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       name,
		Email:      eptr,
		Provider:   s.provider,
		ProviderID: info.GetSubject(),
	})
	return err
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.authn.ServeHTTP(w, r)
}