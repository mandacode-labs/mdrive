package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Unlink removes the entry `name` from directory `parent`.
// nlink==0 destroys the inode. Linux vfs_unlink.
func (v *vfs) Unlink(ctx context.Context, parent *Dentry, name string) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: unlink requires parent and name")
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInternal, "vfs: unlink parent is not a directory")
	}

	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return err
	}
	if dentry.Node.Kind() == NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: cannot unlink a directory; use Rmdir or Remove")
	}
	return v.nodeOp.Unlink(ctx, dentry)
}
