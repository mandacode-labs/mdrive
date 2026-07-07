package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// unlink — Linux vfs_unlink.
func (v *vfs) unlink(ctx context.Context, parent *fs.Dentry, name string) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: unlink requires parent and name")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInternal, "fs: unlink parent is not a directory")
	}
	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return err
	}
	if dentry.Node.Kind() == fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: cannot unlink a directory")
	}
	return v.nodeOp.Unlink(ctx, dentry)
}
