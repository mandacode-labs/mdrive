package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// link — Linux vfs_link.
func (v *vfs) Link(ctx context.Context, oldDentry *fs.Dentry, linkParent *fs.Dentry, linkName string) error {
	if oldDentry == nil || linkParent == nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: link requires oldDentry, linkParent")
	}
	if linkName == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: link name must be non-empty")
	}
	if linkParent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: link parent is not a directory")
	}
	if oldDentry.Node.Kind() == fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: cannot hardlink a directory")
	}
	return v.nodeOp.Link(ctx, oldDentry, linkParent, linkName)
}
