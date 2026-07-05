// Package permission defines the Authorizer interface used to
// gate access across the app. The OpenFGA implementation lives
// in openfga.go and is wired in by the app bootstrap.
package permission

import "context"

// Action is a typed enum for the OpenFGA relation strings the
// app actually issues. Catches typos at compile time and keeps
// handler signatures self-documenting.
type Action string

const (
	ActionView   Action = "can_view"
	ActionEdit   Action = "can_edit"
	ActionDelete Action = "can_delete"
	ActionManage Action = "can_manage"
	ActionShare  Action = "can_share"
)

// ObjectType is a typed enum for the OpenFGA object type names.
// Handlers must pass one of these (ObjectTypeDrive is the only
// value referenced from outside the permission package;
// ObjectTypeUser is internal to the OpenFGA tuple encoding).
type ObjectType string

const (
	ObjectTypeDrive ObjectType = "drive"
	ObjectTypeUser  ObjectType = "user"
)

// Authorizer is the single capability the rest of the app
// depends on. The implementation is wired at boot (FGAChecker
// in production; mock in tests).
type Authorizer interface {
	Grant(ctx context.Context, user, relation string, objectType ObjectType, objectID string) error
	Revoke(ctx context.Context, user, relation string, objectType ObjectType, objectID string) error
	Check(ctx context.Context, user string, perm Action, objectType ObjectType, objectID string) (bool, error)
	ListObjects(ctx context.Context, user string, perm Action, objectType ObjectType) ([]string, error)
}
