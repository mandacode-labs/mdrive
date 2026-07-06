package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Link adds a hard link: a second directory entry under
// `linkName` in `linkParent` pointing at the same inode as
// `oldDentry`. Linux vfs_link.
func (v *vfs) Link(ctx context.Context, oldDentry *Dentry, linkParent *Dentry, linkName string) error {
	if oldDentry == nil || linkParent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link requires oldDentry and linkParent")
	}
	if linkName == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link name must be non-empty")
	}
	if linkParent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link parent is not a directory")
	}
	if oldDentry.Node.Kind() == NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: cannot hardlink a directory")
	}
	return v.nodeOp.Link(ctx, oldDentry, linkParent.Node, linkName)
}
