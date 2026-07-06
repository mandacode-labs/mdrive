package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Rmdir removes an empty directory entry from `parent`.
// POSIX semantics: refuse non-empty. Linux vfs_rmdir.
func (v *vfs) Rmdir(ctx context.Context, parent *Dentry, name string) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: rmdir requires parent and name")
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInternal, "vfs: rmdir parent is not a directory")
	}

	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return err
	}
	if dentry.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: target is not a directory")
	}
	return v.nodeOp.Rmdir(ctx, dentry)
}
