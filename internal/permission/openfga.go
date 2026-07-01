package permission

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
)

const (
	objectTypeDrive = "drive"
	objectTypeUser  = "user"

	relationOwner  = "owner"
	relationEditor = "editor"
	relationViewer = "viewer"
)

// Action is a typed enum for OpenFGA relation strings. Using a
// named type catches typos at compile time and lets signatures
// document intent without losing string compatibility (Action's
// underlying type is string, so the OpenFGA SDK accepts it via
// implicit conversion at the boundary).
type Action string

const (
	ActionView   Action = "can_view"
	ActionEdit   Action = "can_edit"
	ActionDelete Action = "can_delete"
	ActionManage Action = "can_manage"
	ActionShare  Action = "can_share"
)

const (
	ObjectTypeDrive = objectTypeDrive
	ObjectTypeUser  = objectTypeUser
	RelationOwner   = relationOwner
	RelationEditor  = relationEditor
	RelationViewer  = relationViewer
)

// Authorizer grants, revokes, and checks OpenFGA relations.
type Authorizer interface {
	Grant(ctx context.Context, user, relation, objectType, objectID string) error
	Revoke(ctx context.Context, user, relation, objectType, objectID string) error
	Check(ctx context.Context, user string, perm Action, objectType, objectID string) (bool, error)
	ListObjects(ctx context.Context, user string, perm Action, objectType string) ([]string, error)
}

// FGAChecker implements Authorizer using an OpenFGA client.
type FGAChecker struct {
	client *client.OpenFgaClient
}

// AuthMode selects the OpenFGA authentication strategy.
type AuthMode string

const (
	// AuthModeAPIToken uses a static API token (api_token).
	AuthModeAPIToken AuthMode = "api_token"
	// AuthModeClientCredentials uses OAuth client credentials flow
	// (client_id + client_secret + token_issuer + audience).
	AuthModeClientCredentials AuthMode = "client_credentials"
	// AuthModeNone disables authentication (development only).
	AuthModeNone AuthMode = "none"
)

var ErrInvalidAuthMode = errorx.New(errorx.KindBadRequest, "permission: invalid openfga auth_mode (allowed: api_token, client_credentials, none)")

var ErrPermission = errorx.New(errorx.KindForbidden, "permission: denied")

// NopAuthorizer permits every check. Use this explicitly in
// development and test code where no real backend is wired; the
// nil-tolerance in Require was removed because a misconfigured
// Authorizer silently allowing access is the wrong default for an
// auth library.
type NopAuthorizer struct{}

// Check always returns (true, nil).
func (NopAuthorizer) Check(context.Context, string, Action, string, string) (bool, error) {
	return true, nil
}

// ListObjects always returns an empty list.
func (NopAuthorizer) ListObjects(context.Context, string, Action, string) ([]string, error) {
	return nil, nil
}

// Grant is a no-op.
func (NopAuthorizer) Grant(context.Context, string, string, string, string) error { return nil }

// Revoke is a no-op.
func (NopAuthorizer) Revoke(context.Context, string, string, string, string) error { return nil }

// Require is the canonical permission check. It returns ErrPermission
// (wrapped with a hint) if the user lacks the permission, the
// authorizer's own error if the call failed, or nil on success.
//
// Require panics if a is nil. Pass permission.NopAuthorizer
// explicitly when no real backend is wired.
func Require(ctx context.Context, a Authorizer, userID string, perm Action, objectType, objectID string) error {
	if a == nil {
		panic("permission.Require: Authorizer is nil; use permission.NopAuthorizer for development")
	}
	allowed, err := a.Check(ctx, userID, perm, objectType, objectID)
	if err != nil {
		return errorx.Wrap(err, "permission: check (perm=%s, type=%s, id=%s)", perm, objectType, objectID)
	}
	if !allowed {
		return errorx.Wrap(ErrPermission, "permission: denied (perm=%s, type=%s, id=%s)", perm, objectType, objectID)
	}
	return nil
}

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
		return nil, errorx.New(errorx.KindBadRequest, `openfga: store_id is required; create one with: fga store create --name "mdrive"`)
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
		return nil, errorx.Wrap(err, "openfga: create client (api_url=%s)", cfg.APIURL)
	}

	if _, err := c.GetStore(ctx).Execute(); err != nil {
		return nil, errorx.Wrap(err, "openfga: store not found (store_id=%s)", cfg.StoreID)
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

// buildCredentials validates AuthMode and constructs the appropriate
// SDK credentials. Extracted for testability.
func buildCredentials(cfg Config, mode AuthMode) (*credentials.Credentials, error) {
	if mode != AuthModeAPIToken && mode != AuthModeClientCredentials && mode != AuthModeNone {
		return nil, ErrInvalidAuthMode
	}
	switch mode {
	case AuthModeAPIToken:
		if cfg.APIToken == "" {
			return nil, errorx.New(errorx.KindBadRequest, "openfga: auth_mode=api_token requires api_token")
		}
		if cfg.ClientID != "" || cfg.ClientSecret != "" {
			return nil, errorx.New(errorx.KindBadRequest, "openfga: auth_mode=api_token does not accept client_id/client_secret")
		}
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: cfg.APIToken,
			},
		})
		if err != nil {
			return nil, errorx.Wrap(err, "openfga: credentials (auth_mode=%s)", cfg.AuthMode)
		}
		return creds, nil
	case AuthModeClientCredentials:
		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return nil, errorx.New(errorx.KindBadRequest, "openfga: auth_mode=client_credentials requires client_id and client_secret")
		}
		if cfg.TokenIssuer == "" || cfg.Audience == "" {
			return nil, errorx.New(errorx.KindBadRequest, "openfga: auth_mode=client_credentials requires token_issuer and audience")
		}
		if cfg.APIToken != "" {
			return nil, errorx.New(errorx.KindBadRequest, "openfga: auth_mode=client_credentials does not accept api_token")
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
			return nil, errorx.Wrap(err, "openfga: credentials (auth_mode=%s)", cfg.AuthMode)
		}
		return creds, nil
	case AuthModeNone:
		if cfg.APIToken != "" || cfg.ClientID != "" || cfg.ClientSecret != "" {
			return nil, errorx.New(errorx.KindBadRequest, "openfga: auth_mode=none does not accept any credentials")
		}
		return nil, nil
	}
	return nil, ErrInvalidAuthMode
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

// Grant creates a (user, relation, object) tuple.
func (c *FGAChecker) Grant(ctx context.Context, user, relation, objectType, objectID string) error {
	_, err := c.client.Write(ctx).Body(client.ClientWriteRequest{
		Writes: []client.ClientTupleKey{{
			User:     userObject(user),
			Relation: relation,
			Object:   objectRef(objectType, objectID),
		}},
	}).Execute()
	return err
}

// Revoke deletes a (user, relation, object) tuple.
func (c *FGAChecker) Revoke(ctx context.Context, user, relation, objectType, objectID string) error {
	_, err := c.client.Write(ctx).Body(client.ClientWriteRequest{
		Deletes: []client.ClientTupleKeyWithoutCondition{{
			User:     userObject(user),
			Relation: relation,
			Object:   objectRef(objectType, objectID),
		}},
	}).Execute()
	return err
}

// Check returns true if the user has the given permission on the object.
func (c *FGAChecker) Check(ctx context.Context, user string, perm Action, objectType, objectID string) (bool, error) {
	resp, err := c.client.Check(ctx).Body(client.ClientCheckRequest{
		User:     userObject(user),
		Relation: string(perm),
		Object:   objectRef(objectType, objectID),
	}).Execute()
	if err != nil {
		return false, err
	}
	return resp.GetAllowed(), nil
}

// ListObjects returns the IDs of objects of the given type that the user has the given permission on.
func (c *FGAChecker) ListObjects(ctx context.Context, user string, perm Action, objectType string) ([]string, error) {
	resp, err := c.client.ListObjects(ctx).Body(client.ClientListObjectsRequest{
		User:     userObject(user),
		Relation: string(perm),
		Type:     objectType,
	}).Execute()
	if err != nil {
		return nil, err
	}
	return resp.GetObjects(), nil
}

func userObject(userID string) string {
	return objectTypeUser + ":" + userID
}

func objectRef(objectType, objectID string) string {
	return objectType + ":" + objectID
}

// DriveObjectRef returns the OpenFGA object reference for a drive.
func DriveObjectRef(driveID string) string {
	return objectRef(objectTypeDrive, driveID)
}

// UserObjectRef returns the OpenFGA user reference for a user.
func UserObjectRef(userID string) string {
	return userObject(userID)
}

// GrantOwner grants the owner relation.
func GrantOwner(ctx context.Context, a Authorizer, userID, driveID string) error {
	return a.Grant(ctx, userID, relationOwner, objectTypeDrive, driveID)
}

// GrantEditor grants the editor relation.
func GrantEditor(ctx context.Context, a Authorizer, userID, driveID string) error {
	return a.Grant(ctx, userID, relationEditor, objectTypeDrive, driveID)
}

// GrantViewer grants the viewer relation.
func GrantViewer(ctx context.Context, a Authorizer, userID, driveID string) error {
	return a.Grant(ctx, userID, relationViewer, objectTypeDrive, driveID)
}

// RevokeAllRelations revokes all relations for a user on a drive.
func RevokeAllRelations(ctx context.Context, a Authorizer, userID, driveID string) error {
	for _, rel := range []string{relationOwner, relationEditor, relationViewer} {
		if err := a.Revoke(ctx, userID, rel, objectTypeDrive, driveID); err != nil {
			return err
		}
	}
	return nil
}

var _ Authorizer = (*FGAChecker)(nil)
