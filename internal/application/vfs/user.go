package vfs

import "context"

// UserService defines the subset of user operations needed by the VFS layer.
type UserService interface {
	ResolveUID(ctx context.Context, systemID string) (int, error)
	ResolveUIDAndGIDs(ctx context.Context, systemID string) (uid int, gids []int, err error)
}
