package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Unmount — Linux umount(2) counterpart for bind mounts.
// Removes the mount entry from `parent` and destroys the
// mount node. Refuses if the entry isn't a mount.
func (v *vfs) Unmount(ctx context.Context, parent *fs.Dentry, name string) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: unmount requires parent and name")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: unmount parent is not a directory")
	}
	dentry, err := v.nodeOp.Lookup(ctx, parent, name)
	if err != nil {
		return err
	}
	if dentry.Node.Kind() != fs.NodeKindMount {
		return errorx.New(errorx.KindInvalidArgument, "fs: target is not a mount")
	}
	return v.nodeOp.Unlink(ctx, dentry)
}
