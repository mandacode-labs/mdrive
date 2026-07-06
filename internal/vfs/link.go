package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Link creates a hardlink: linkPath becomes a second directory
// entry pointing at the same inode as srcPath. Linux vfs_link.
func (v *vfs) Link(ctx context.Context, driveID string, srcPath string, linkPath string) error {
	srcTarget, err := v.resolveTarget(ctx, driveID, srcPath, permission.ActionView)
	if err != nil {
		return err
	}
	if srcTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: source path has no parent")
	}
	if srcTarget.Node.Kind() == NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: cannot hardlink a directory")
	}
	dstTarget, err := v.resolveTarget(ctx, driveID, linkPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	if dstTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link path has no parent")
	}
	return v.nodeOp.Link(ctx, srcTarget, dstTarget.Parent, dstTarget.Name)
}