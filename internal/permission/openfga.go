package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

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

// Permission is a typed enum for OpenFGA relation strings.
// Using a named type catches typos at compile time and lets
// signatures document intent without losing string compatibility
// (Permission's underlying type is string, so the OpenFGA SDK
// accepts it via implicit conversion at the boundary).
type Permission string

const (
	PermissionView   Permission = "can_view"
	PermissionEdit   Permission = "can_edit"
	PermissionDelete Permission = "can_delete"
	PermissionManage Permission = "can_manage"
	PermissionShare  Permission = "can_share"
)

// Exported constants.
const (
	ObjectTypeDrive = objectTypeDrive
	ObjectTypeUser  = objectTypeUser
	RelationOwner   = relationOwner
	RelationEditor  = relationEditor
	RelationViewer  = relationViewer
)

// Checker grants, revokes, and checks OpenFGA relations.
type Checker interface {
	Grant(ctx context.Context, user, relation, objectType, objectID string) error
	Revoke(ctx context.Context, user, relation, objectType, objectID string) error
	Check(ctx context.Context, user string, perm Permission, objectType, objectID string) (bool, error)
	ListObjects(ctx context.Context, user string, perm Permission, objectType string) ([]string, error)
}

// OpenFGAChecker implements Checker using an OpenFGA client.
type OpenFGAChecker struct {
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

// ErrInvalidAuthMode is returned when an unknown AuthMode is provided.
var ErrInvalidAuthMode = fmt.Errorf("permission: invalid openfga auth_mode (allowed: api_token, client_credentials, none)")

// ErrPermission is the single permission-denied sentinel. All
// packages that need to report "not allowed" return this directly
// or wrap it; consumers test with errors.Is(err, permission.ErrPermission).
var ErrPermission = errors.New("permission: denied")

// Require is the canonical permission check. It returns ErrPermission
// (wrapped with a hint) if the user lacks the permission, the
// checker's own error if the call failed, or nil on success.
//
// A nil checker (development mode) returns nil. This is the
// single point of nil-tolerance so call sites don't have to
// reproduce the same `if c == nil { return nil }` guard.
func Require(ctx context.Context, c Checker, userID string, perm Permission, objectType, objectID string) error {
	if c == nil {
		return nil
	}
	allowed, err := c.Check(ctx, userID, perm, objectType, objectID)
	if err != nil {
		return fmt.Errorf("permission: check %s on %s:%s: %w", perm, objectType, objectID, err)
	}
	if !allowed {
		return fmt.Errorf("%s on %s:%s: %w", perm, objectType, objectID, ErrPermission)
	}
	return nil
}

// Config for OpenFGAChecker.
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
	Timeout              time.Duration
}

// NewOpenFGAChecker creates a new OpenFGAChecker.
// StoreID is required. Use fga CLI to create it: fga store create --name "mdrive"
// AuthorizationModelID is optional. If empty, the embedded model is written and used.
//
// AuthMode is required and must be one of AuthModeAPIToken, AuthModeClientCredentials,
// or AuthModeNone. Mixing credentials across modes is a configuration error.
func NewOpenFGAChecker(ctx context.Context, cfg Config) (*OpenFGAChecker, error) {
	if cfg.StoreID == "" {
		return nil, fmt.Errorf("openfga: store_id is required; create one with: fga store create --name \"mdrive\"")
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
		return nil, fmt.Errorf("openfga: create client: %w", err)
	}

	if _, err := c.GetStore(ctx).Execute(); err != nil {
		return nil, fmt.Errorf("openfga: store not found (%s): %w", cfg.StoreID, err)
	}

	if cfg.AuthorizationModelID == "" {
		modelID, err := writeModel(ctx, c)
		if err != nil {
			return nil, err
		}
		if err := c.SetAuthorizationModelId(modelID); err != nil {
			return nil, fmt.Errorf("openfga: set model id: %w", err)
		}
	}

	return &OpenFGAChecker{client: c}, nil
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
			return nil, fmt.Errorf("openfga: auth_mode=api_token requires api_token")
		}
		if cfg.ClientID != "" || cfg.ClientSecret != "" {
			return nil, fmt.Errorf("openfga: auth_mode=api_token does not accept client_id/client_secret")
		}
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: cfg.APIToken,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("openfga: credentials: %w", err)
		}
		return creds, nil
	case AuthModeClientCredentials:
		if cfg.ClientID == "" || cfg.ClientSecret == "" {
			return nil, fmt.Errorf("openfga: auth_mode=client_credentials requires client_id and client_secret")
		}
		if cfg.TokenIssuer == "" || cfg.Audience == "" {
			return nil, fmt.Errorf("openfga: auth_mode=client_credentials requires token_issuer and audience")
		}
		if cfg.APIToken != "" {
			return nil, fmt.Errorf("openfga: auth_mode=client_credentials does not accept api_token")
		}
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodClientCredentials,
			Config: &credentials.Config{
				ClientCredentialsClientId:       cfg.ClientID,
				ClientCredentialsClientSecret:   cfg.ClientSecret,
				ClientCredentialsApiTokenIssuer: cfg.TokenIssuer,
				ClientCredentialsApiAudience:    cfg.Audience,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("openfga: credentials: %w", err)
		}
		return creds, nil
	case AuthModeNone:
		if cfg.APIToken != "" || cfg.ClientID != "" || cfg.ClientSecret != "" {
			return nil, fmt.Errorf("openfga: auth_mode=none does not accept any credentials")
		}
		return nil, nil
	}
	return nil, ErrInvalidAuthMode
}

// writeModel writes the embedded authorization model and returns the new model ID.
func writeModel(ctx context.Context, c *client.OpenFgaClient) (string, error) {
	var req client.ClientWriteAuthorizationModelRequest
	if err := json.Unmarshal(ModelJSON, &req); err != nil {
		return "", fmt.Errorf("openfga: decode model: %w", err)
	}
	resp, err := c.WriteAuthorizationModel(ctx).Body(req).Execute()
	if err != nil {
		return "", fmt.Errorf("openfga: write model: %w", err)
	}
	return resp.AuthorizationModelId, nil
}

// Grant creates a (user, relation, object) tuple.
func (c *OpenFGAChecker) Grant(ctx context.Context, user, relation, objectType, objectID string) error {
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
func (c *OpenFGAChecker) Revoke(ctx context.Context, user, relation, objectType, objectID string) error {
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
func (c *OpenFGAChecker) Check(ctx context.Context, user string, perm Permission, objectType, objectID string) (bool, error) {
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
func (c *OpenFGAChecker) ListObjects(ctx context.Context, user string, perm Permission, objectType string) ([]string, error) {
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
func GrantOwner(ctx context.Context, c Checker, userID, driveID string) error {
	return c.Grant(ctx, userID, relationOwner, objectTypeDrive, driveID)
}

// GrantEditor grants the editor relation.
func GrantEditor(ctx context.Context, c Checker, userID, driveID string) error {
	return c.Grant(ctx, userID, relationEditor, objectTypeDrive, driveID)
}

// GrantViewer grants the viewer relation.
func GrantViewer(ctx context.Context, c Checker, userID, driveID string) error {
	return c.Grant(ctx, userID, relationViewer, objectTypeDrive, driveID)
}

// RevokeAllRelations revokes all relations for a user on a drive.
func RevokeAllRelations(ctx context.Context, c Checker, userID, driveID string) error {
	for _, rel := range []string{relationOwner, relationEditor, relationViewer} {
		if err := c.Revoke(ctx, userID, rel, objectTypeDrive, driveID); err != nil {
			return err
		}
	}
	return nil
}

var _ Checker = (*OpenFGAChecker)(nil)
