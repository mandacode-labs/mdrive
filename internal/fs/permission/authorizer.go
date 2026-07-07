// Package permission defines the Authorizer interface used to
// gate access across the app. The OpenFGA implementation
// lives in openfga.go.
package permission

import "context"

// Action is a typed enum for the OpenFGA relation strings.
type Action string

const (
	ActionView   Action = "can_view"
	ActionEdit   Action = "can_edit"
	ActionDelete Action = "can_delete"
	ActionManage Action = "can_manage"
	ActionShare  Action = "can_share"
)

// ObjectType is a typed enum for the OpenFGA object types.
type ObjectType string

const (
	ObjectTypeDrive ObjectType = "drive"
	ObjectTypeUser  ObjectType = "user"
)

// Authorizer is the single capability the rest of the app
// depends on.
type Authorizer interface {
	Grant(ctx context.Context, user, relation string, objectType ObjectType, objectID string) error
	Revoke(ctx context.Context, user, relation string, objectType ObjectType, objectID string) error
	Check(ctx context.Context, user string, perm Action, objectType ObjectType, objectID string) (bool, error)
	ListObjects(ctx context.Context, user string, perm Action, objectType ObjectType) ([]string, error)
}
