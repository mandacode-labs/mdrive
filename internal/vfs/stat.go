package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Stat follows symlinks and returns the resolved node. Linux
// vfs_stat / stat(2).
func (v *vfs) Stat(ctx context.Context, driveID string, path string) (*Node, error) {
	dentry, err := v.resolveTarget(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return nil, err
	}
	return dentry.Node, nil
}