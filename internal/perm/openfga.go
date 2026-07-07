package perm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
)

// OpenFGAService implements Service using an OpenFGA client.
type OpenFGAService struct {
	client *client.OpenFgaClient
}

// AuthMode selects the OpenFGA authentication strategy.
type AuthMode string

const (
	AuthModeAPIToken          AuthMode = "api_token"
	AuthModeClientCredentials AuthMode = "client_credentials"
	AuthModeNone              AuthMode = "none"
)

// Config for OpenFGAService.
type Config struct {
	AuthMode             AuthMode
	APIURL               string
	StoreID              string
	AuthorizationModelID string
	APIToken             string
	ClientID             string
	ClientSecret         string
	TokenIssuer          string
	Audience             string
	Scopes               []string
	Timeout              time.Duration
}

// NewOpenFGAService creates a new OpenFGA-backed Service.
//
// StoreID is required. AuthorizationModelID is optional — if
// empty, the embedded model is written and used.
func NewOpenFGAService(ctx context.Context, cfg Config) (*OpenFGAService, error) {
	if cfg.StoreID == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, `openfga: store_id is required; create one with: fga store create --name "mdrive"`)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	mode := cfg.AuthMode
	if mode == "" {
		mode = AuthModeAPIToken
	}
	creds, err := buildCredentials(cfg, mode)
	if err != nil {
		return nil, err
	}

	c, err := client.NewSdkClient(&client.ClientConfiguration{
		ApiUrl:               cfg.APIURL,
		StoreId:              cfg.StoreID,
		AuthorizationModelId: cfg.AuthorizationModelID,
		Credentials:          creds,
		HTTPClient:           &http.Client{Timeout: timeout},
	})
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("openfga: create client (api_url=%s)", cfg.APIURL))
	}

	if _, err := c.GetStore(ctx).Execute(); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("openfga: store not found (store_id=%s)", cfg.StoreID))
	}

	if cfg.AuthorizationModelID == "" {
		modelID, err := writeModel(ctx, c)
		if err != nil {
			return nil, err
		}
		if err := c.SetAuthorizationModelId(modelID); err != nil {
			return nil, errorx.Wrap(err, "openfga: set model id")
		}
	}

	return &OpenFGAService{client: c}, nil
}

// Check returns nil if user holds `action` on (objectType, objectID).
func (s *OpenFGAService) Check(ctx context.Context, userID string, action Action, objectType ObjectType, objectID string) error {
	resp, err := s.client.Check(ctx).Body(client.ClientCheckRequest{
		User:     "user:" + userID,
		Relation: string(action),
		Object:   string(objectType) + ":" + objectID,
	}).Execute()
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("openfga: check (user=%s, action=%s, type=%s, id=%s)", userID, action, objectType, objectID))
	}
	if !resp.GetAllowed() {
		return errorx.New(errorx.KindPermissionDenied, fmt.Sprintf("perm: denied (user=%s, action=%s, type=%s, id=%s)", userID, action, objectType, objectID))
	}
	return nil
}

func buildCredentials(cfg Config, mode AuthMode) (*credentials.Credentials, error) {
	if mode != AuthModeAPIToken && mode != AuthModeClientCredentials && mode != AuthModeNone {
		return nil, errorx.New(errorx.KindInvalidArgument, "permission: invalid openfga auth_mode (allowed: api_token, client_credentials, none)")
	}
	switch mode {
	case AuthModeAPIToken:
		if cfg.APIToken == "" {
			return nil, errorx.New(errorx.KindInvalidArgument, "openfga: auth_mode=api_token requires api_token")
		}
		if cfg.ClientID != "" || cfg.ClientSecret != "" {
			return nil, errorx.New(errorx.KindInvalidArgument, "openfga: auth_mode=api_token does not accept client_id/client_secret")
		}
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: cfg.APIToken,
			},
		})
		if err != nil {
			return nil, errorx.Wrap(err, fmt.Sprintf("openfga: credentials (auth_mode=%s)", mode))
		}
		return creds, nil
	case AuthModeClientCredentials:
		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return nil, errorx.New(errorx.KindInvalidArgument, "openfga: auth_mode=client_credentials requires client_id and client_secret")
		}
		if cfg.TokenIssuer == "" || cfg.Audience == "" {
			return nil, errorx.New(errorx.KindInvalidArgument, "openfga: auth_mode=client_credentials requires token_issuer and audience")
		}
		if cfg.APIToken != "" {
			return nil, errorx.New(errorx.KindInvalidArgument, "openfga: auth_mode=client_credentials does not accept api_token")
		}
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodClientCredentials,
			Config: &credentials.Config{
				ClientCredentialsClientId:       cfg.ClientID,
				ClientCredentialsClientSecret:   cfg.ClientSecret,
				ClientCredentialsApiTokenIssuer: cfg.TokenIssuer,
				ClientCredentialsApiAudience:    cfg.Audience,
				ClientCredentialsScopes:         strings.Join(cfg.Scopes, " "),
			},
		})
		if err != nil {
			return nil, errorx.Wrap(err, fmt.Sprintf("openfga: credentials (auth_mode=%s)", mode))
		}
		return creds, nil
	case AuthModeNone:
		if cfg.APIToken != "" || cfg.ClientID != "" || cfg.ClientSecret != "" {
			return nil, errorx.New(errorx.KindInvalidArgument, "openfga: auth_mode=none does not accept any credentials")
		}
		return nil, nil
	}
	return nil, errorx.New(errorx.KindInvalidArgument, "permission: invalid openfga auth_mode (allowed: api_token, client_credentials, none)")
}

func writeModel(ctx context.Context, c *client.OpenFgaClient) (string, error) {
	var req client.ClientWriteAuthorizationModelRequest
	if err := json.Unmarshal(ModelJSON, &req); err != nil {
		return "", errorx.Wrap(err, "openfga: decode model")
	}
	resp, err := c.WriteAuthorizationModel(ctx).Body(req).Execute()
	if err != nil {
		return "", errorx.Wrap(err, "openfga: write model")
	}
	return resp.AuthorizationModelId, nil
}