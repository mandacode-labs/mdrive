// Package permission provides authorization primitives backed by OpenFGA.
package permission

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/openfga/go-sdk/client"
)

// Object types used in the OpenFGA model.
const (
	ObjectTypeDrive = "drive"
	ObjectTypeUser  = "user"
)

// Relations on the drive object.
const (
	RelationOwner  = "owner"
	RelationEditor = "editor"
	RelationViewer = "viewer"
)

// Permissions on the drive object.
const (
	PermissionView   = "can_view"
	PermissionEdit   = "can_edit"
	PermissionDelete = "can_delete"
	PermissionManage = "can_manage"
	PermissionShare  = "can_share"
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
	Timeout              time.Duration
}

// NewOpenFGAChecker creates a new OpenFGAChecker.
func NewOpenFGAChecker(_ context.Context, cfg Config) (*OpenFGAChecker, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	c, err := client.NewSdkClient(&client.ClientConfiguration{
		ApiUrl:               cfg.APIURL,
		StoreId:              cfg.StoreID,
		AuthorizationModelId: cfg.AuthorizationModelID,
		HTTPClient:           &http.Client{Timeout: timeout},
	})
	if err != nil {
		return nil, fmt.Errorf("openfga: create client: %w", err)
	}
	return &OpenFGAChecker{client: c}, nil
}

// Grant creates a (user, relation, object) tuple.
func (c *OpenFGAChecker) Grant(ctx context.Context, user, relation, objectType, objectID string) error {
	body := client.ClientWriteRequest{
		Writes: []client.ClientTupleKey{
			{
				User:     userObject(user),
				Relation: relation,
				Object:   objectRef(objectType, objectID),
			},
		},
	}
	_, err := c.client.Write(ctx).Body(body).Execute()
	return err
}

// Revoke deletes a (user, relation, object) tuple.
func (c *OpenFGAChecker) Revoke(ctx context.Context, user, relation, objectType, objectID string) error {
	body := client.ClientWriteRequest{
		Deletes: []client.ClientTupleKeyWithoutCondition{
			{
				User:     userObject(user),
				Relation: relation,
				Object:   objectRef(objectType, objectID),
			},
		},
	}
	_, err := c.client.Write(ctx).Body(body).Execute()
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

// userObject formats a user ID into the OpenFGA user object form.
func userObject(userID string) string {
	return fmt.Sprintf("%s:%s", ObjectTypeUser, userID)
}

// objectRef formats a (type, id) pair into the OpenFGA object reference form.
func objectRef(objectType, objectID string) string {
	return fmt.Sprintf("%s:%s", objectType, objectID)
}

// DriveObjectRef returns the OpenFGA object reference for a drive.
func DriveObjectRef(driveID string) string {
	return objectRef(ObjectTypeDrive, driveID)
}

// UserObjectRef returns the OpenFGA user reference for a user.
func UserObjectRef(userID string) string {
	return userObject(userID)
}

// GrantOwner grants the owner relation. Call this when a drive is created.
func GrantOwner(ctx context.Context, c Checker, userID, driveID string) error {
	return c.Grant(ctx, userID, RelationOwner, ObjectTypeDrive, driveID)
}

// GrantEditor grants the editor relation.
func GrantEditor(ctx context.Context, c Checker, userID, driveID string) error {
	return c.Grant(ctx, userID, RelationEditor, ObjectTypeDrive, driveID)
}

// GrantViewer grants the viewer relation.
func GrantViewer(ctx context.Context, c Checker, userID, driveID string) error {
	return c.Grant(ctx, userID, RelationViewer, ObjectTypeDrive, driveID)
}

// RevokeAllRelations revokes all relations for a user on a drive.
func RevokeAllRelations(ctx context.Context, c Checker, userID, driveID string) error {
	for _, rel := range []string{RelationOwner, RelationEditor, RelationViewer} {
		if err := c.Revoke(ctx, userID, rel, ObjectTypeDrive, driveID); err != nil {
			return err
		}
	}
	return nil
}

// Compile-time check.
var _ Checker = (*OpenFGAChecker)(nil)
