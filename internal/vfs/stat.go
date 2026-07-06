package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
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

// Lstat does not follow the final symlink. Linux vfs_lstat /
// lstat(2).
func (v *vfs) Lstat(ctx context.Context, driveID string, path string) (*Node, error) {
	if _, err := v.resolveTarget(ctx, driveID, path, permission.ActionView); err != nil {
		return nil, err
	}
	// Lstat is the resolveTarget result before the symlink
	// follow step. We approximate this by resolving the parent
	// then Looking up the basename directly.
	parent, name, err := v.resolveParent(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return nil, err
	}
	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return nil, err
	}
	return dentry.Node, nil
}

// Readlink returns the symlink target's inode id as a string.
// Linux vfs_readlink.
func (v *vfs) Readlink(ctx context.Context, driveID string, path string) (string, error) {
	parent, name, err := v.resolveParent(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return "", err
	}
	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return "", err
	}
	if dentry.Node.Kind() != NodeKindSymlink {
		return "", errorx.New(errorx.KindInvalidArgument, "vfs: not a symlink")
	}
	var sc contentSymlinkContent
	if err := json.Unmarshal(dentry.Node.Data(), &sc); err != nil {
		return "", errorx.Wrap(err, "vfs: invalid symlink content")
	}
	return sc.Target, nil
}
