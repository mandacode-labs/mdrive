package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Unlink — Linux vfs_unlink. Refuses directories; use Rmdir.
func (v *vfs) Unlink(ctx context.Context, parent *fs.Dentry, name string) error {
	return v.remove(ctx, parent, name, false)
}

// Rmdir — Linux vfs_rmdir. Only directories; use Unlink otherwise.
func (v *vfs) Rmdir(ctx context.Context, parent *fs.Dentry, name string) error {
	return v.remove(ctx, parent, name, true)
}

// remove is the shared body of Unlink and Rmdir — mirrors
// how Linux's vfs_unlink / vfs_rmdir share their dir-hash
// bookkeeping. `removeDir` toggles the kind check.
func (v *vfs) remove(ctx context.Context, parent *fs.Dentry, name string, removeDir bool) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: remove requires parent and name")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInternal, "fs: remove parent is not a directory")
	}
	dentry, err := v.nodeOp.Lookup(ctx, parent, name)
	if err != nil {
		return err
	}
	isDir := dentry.Node.Kind() == fs.NodeKindDirectory
	if removeDir && !isDir {
		return errorx.New(errorx.KindInvalidArgument, "fs: target is not a directory")
	}
	if !removeDir && isDir {
		return errorx.New(errorx.KindInvalidArgument, "fs: cannot unlink a directory")
	}
	if removeDir {
		return v.nodeOp.Rmdir(ctx, dentry)
	}
	return v.nodeOp.Unlink(ctx, dentry)
}
