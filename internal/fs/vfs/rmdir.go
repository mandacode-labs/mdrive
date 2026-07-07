package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// rmdir — Linux vfs_rmdir.
func (v *vfs) rmdir(ctx context.Context, parent *fs.Dentry, name string) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: rmdir requires parent and name")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInternal, "fs: rmdir parent is not a directory")
	}
	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return err
	}
	if dentry.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: target is not a directory")
	}
	return v.nodeOp.Rmdir(ctx, dentry)
}
