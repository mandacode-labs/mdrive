package permission

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

// FGAChecker implements Authorizer using an OpenFGA client.
type FGAChecker struct {
	client *client.OpenFgaClient
}

// AuthMode selects the OpenFGA authentication strategy.
type AuthMode string

const (
	AuthModeAPIToken          AuthMode = "api_token"
	AuthModeClientCredentials AuthMode = "client_credentials"
	AuthModeNone              AuthMode = "none"
)

// Config for FGAChecker.
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

// NewFGAChecker creates a new FGAChecker.
// StoreID is required. Use fga CLI to create it: fga store create --name "mdrive"
// AuthorizationModelID is optional. If empty, the embedded model is written and used.
//
// AuthMode is required and must be one of AuthModeAPIToken, AuthModeClientCredentials,
// or AuthModeNone. Mixing credentials across modes is a configuration error.
func NewFGAChecker(ctx context.Context, cfg Config) (*FGAChecker, error) {
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

	return &FGAChecker{client: c}, nil
}

// Grant creates a (user, relation, object) tuple.
func (c *FGAChecker) Grant(ctx context.Context, user, relation string, objectType ObjectType, objectID string) error {
	_, err := c.client.Write(ctx).Body(client.ClientWriteRequest{
		Writes: []client.ClientTupleKey{{
			User:     "user:" + user,
			Relation: relation,
			Object:   string(objectType) + ":" + objectID,
		}},
	}).Execute()
	return err
}

// Revoke deletes a (user, relation, object) tuple.
func (c *FGAChecker) Revoke(ctx context.Context, user, relation string, objectType ObjectType, objectID string) error {
	_, err := c.client.Write(ctx).Body(client.ClientWriteRequest{
		Deletes: []client.ClientTupleKeyWithoutCondition{{
			User:     "user:" + user,
			Relation: relation,
			Object:   string(objectType) + ":" + objectID,
		}},
	}).Execute()
	return err
}

// Check returns true if the user has the given permission on the object.
func (c *FGAChecker) Check(ctx context.Context, user string, perm Action, objectType ObjectType, objectID string) (bool, error) {
	resp, err := c.client.Check(ctx).Body(client.ClientCheckRequest{
		User:     "user:" + user,
		Relation: string(perm),
		Object:   string(objectType) + ":" + objectID,
	}).Execute()
	if err != nil {
		return false, errorx.Wrap(err, fmt.Sprintf("openfga: check (user=%s, perm=%s, type=%s, id=%s)", user, perm, objectType, objectID))
	}
	return resp.GetAllowed(), nil
}

// ListObjects returns the IDs of objects of the given type that the user has the given permission on.
func (c *FGAChecker) ListObjects(ctx context.Context, user string, perm Action, objectType ObjectType) ([]string, error) {
	resp, err := c.client.ListObjects(ctx).Body(client.ClientListObjectsRequest{
		User:     "user:" + user,
		Relation: string(perm),
		Type:     string(objectType),
	}).Execute()
	if err != nil {
		return nil, err
	}
	return resp.GetObjects(), nil
}

// buildCredentials validates AuthMode and constructs the appropriate
// SDK credentials. Extracted for testability.
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
			return nil, errorx.Wrap(err, fmt.Sprintf("openfga: credentials (auth_mode=%s)", cfg.AuthMode))
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
			return nil, errorx.Wrap(err, fmt.Sprintf("openfga: credentials (auth_mode=%s)", cfg.AuthMode))
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

// writeModel writes the embedded authorization model and returns the new model ID.
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

var _ Authorizer = (*FGAChecker)(nil)
