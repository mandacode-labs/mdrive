// Package permission provides authorization primitives backed by OpenFGA.
package permission

import (
	"context"
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

	permView   = "can_view"
	permEdit   = "can_edit"
	permDelete = "can_delete"
	permManage = "can_manage"
	permShare  = "can_share"

	storeName = "mdrive"

	authMethodAPIToken        = "api_token"
	authHeaderKey             = "Authorization"
	authHeaderValuePrefix     = "Bearer "
)

// Exported permission constants for external use.
const (
	ObjectTypeDrive   = objectTypeDrive
	ObjectTypeUser    = objectTypeUser
	RelationOwner     = relationOwner
	RelationEditor    = relationEditor
	RelationViewer    = relationViewer
	PermissionView    = permView
	PermissionEdit    = permEdit
	PermissionDelete  = permDelete
	PermissionManage  = permManage
	PermissionShare   = permShare
)

// Checker grants, revokes, and checks OpenFGA relations.
type Checker interface {
	Grant(ctx context.Context, user, relation, objectType, objectID string) error
	Revoke(ctx context.Context, user, relation, objectType, objectID string) error
	Check(ctx context.Context, user, permission, objectType, objectID string) (bool, error)
	ListObjects(ctx context.Context, user, permission, objectType string) ([]string, error)
}

// OpenFGAChecker implements Checker using an OpenFGA client.
type OpenFGAChecker struct {
	client *client.OpenFgaClient
}

// Config for OpenFGAChecker.
type Config struct {
	APIURL               string
	StoreID              string
	AuthorizationModelID string
	APIToken             string
	Timeout              time.Duration
}

// NewOpenFGAChecker creates a new OpenFGAChecker.
// StoreID is required. If the store doesn't exist, it is auto-created.
// APIToken enables bearer token authentication with OpenFGA.
func NewOpenFGAChecker(ctx context.Context, cfg Config) (*OpenFGAChecker, error) {
	if cfg.StoreID == "" {
		return nil, fmt.Errorf("openfga: store_id is required when api_url is configured")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	var creds *credentials.Credentials
	if cfg.APIToken != "" {
		var err error
		creds, err = credentials.NewCredentials(credentials.Credentials{
			Method: authMethodAPIToken,
			Config: &credentials.Config{
				ApiToken: cfg.APIToken,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("openfga: credentials: %w", err)
		}
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

	if err := ensureStore(ctx, c); err != nil {
		return nil, err
	}

	if cfg.AuthorizationModelID != "" {
		if err := c.SetAuthorizationModelId(cfg.AuthorizationModelID); err != nil {
			return nil, fmt.Errorf("openfga: set model id: %w", err)
		}
	}

	return &OpenFGAChecker{client: c}, nil
}

// ensureStore verifies the configured store exists, creating it if not.
func ensureStore(ctx context.Context, c *client.OpenFgaClient) error {
	_, err := c.GetStore(ctx).Execute()
	if err == nil {
		return nil
	}
	resp, err := c.CreateStore(ctx).Body(client.ClientCreateStoreRequest{
		Name: storeName,
	}).Execute()
	if err != nil {
		return fmt.Errorf("openfga: create store: %w", err)
	}
	return fmt.Errorf("openfga: store not found, created new store (id=%s). Update your config with this store_id", resp.Id)
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
func (c *OpenFGAChecker) Check(ctx context.Context, user, permission, objectType, objectID string) (bool, error) {
	resp, err := c.client.Check(ctx).Body(client.ClientCheckRequest{
		User:     userObject(user),
		Relation: permission,
		Object:   objectRef(objectType, objectID),
	}).Execute()
	if err != nil {
		return false, err
	}
	return resp.GetAllowed(), nil
}

// ListObjects returns the IDs of objects of the given type that the user has the given permission on.
func (c *OpenFGAChecker) ListObjects(ctx context.Context, user, permission, objectType string) ([]string, error) {
	resp, err := c.client.ListObjects(ctx).Body(client.ClientListObjectsRequest{
		User:     userObject(user),
		Relation: permission,
		Type:     objectType,
	}).Execute()
	if err != nil {
		return nil, err
	}
	return resp.GetObjects(), nil
}

func userObject(userID string) string {
	return fmt.Sprintf("%s:%s", objectTypeUser, userID)
}

func objectRef(objectType, objectID string) string {
	return fmt.Sprintf("%s:%s", objectType, objectID)
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
