package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Walk passes through to the Resolver. Resolver.Walk already does
// the perm check, mount crossing, and symlink follow; vfs.Walk
// is the public-facing alias so callers depend on vfs.VFS, not
// vfs.Resolver directly.
func (v *vfs) Walk(ctx context.Context, driveID string, path string, action permission.Action) (*Dentry, error) {
	return v.resolver.Walk(ctx, driveID, path, action)
}
