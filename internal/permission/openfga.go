package permission

import (
	"context"
	"encoding/json"
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
)

// Exported permission constants.
const (
	ObjectTypeDrive  = objectTypeDrive
	ObjectTypeUser   = objectTypeUser
	RelationOwner    = relationOwner
	RelationEditor   = relationEditor
	RelationViewer   = relationViewer
	PermissionView   = permView
	PermissionEdit   = permEdit
	PermissionDelete = permDelete
	PermissionManage = permManage
	PermissionShare  = permShare
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
// StoreID is required. Use fga CLI to create it: fga store create --name "mdrive"
// AuthorizationModelID is optional. If empty, the embedded model is written and used.
func NewOpenFGAChecker(ctx context.Context, cfg Config) (*OpenFGAChecker, error) {
	if cfg.StoreID == "" {
		return nil, fmt.Errorf("openfga: store_id is required; create one with: fga store create --name \"mdrive\"")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	var creds *credentials.Credentials
	if cfg.APIToken != "" {
		var err error
		creds, err = credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
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
