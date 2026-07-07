// Package perm defines the capability check interface.
//
// The fs.Service and drive.Service consume perm.Service
// directly (consumer-side interface pattern).
package perm

import "context"

// Service is the access-control capability. Check returns
// nil if the user holds `action` on `(objectType, objectID)`,
// or an error (KindPermissionDenied) otherwise.
type Service interface {
	// Check returns nil if the user has `action` on the object,
	// or an error otherwise.
	Check(ctx context.Context, userID string, action Action, objectType ObjectType, objectID string) error
}