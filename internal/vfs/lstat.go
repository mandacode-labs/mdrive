package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Lstat does not follow the final symlink. Linux vfs_lstat /
// lstat(2).
func (v *vfs) Lstat(ctx context.Context, driveID string, path string) (*Node, error) {
	target, err := v.resolveTarget(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return nil, err
	}
	if target.Parent == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: path has no parent")
	}
	dentry, err := v.nodeOp.Lookup(ctx, target.Parent, target.Name)
	if err != nil {
		return nil, err
	}
	return dentry.Node, nil
}