package vfs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Link creates a hard link. linkPath becomes a second directory
// entry pointing at the same inode as srcPath. Linux vfs_link.
func (v *vfs) Link(ctx context.Context, driveID string, srcPath string, linkPath string) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return err
	}
	srcTarget, _, err := v.walkEntry(ctx, startDrive, srcPath, false)
	if err != nil {
		return err
	}
	if srcTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: source path has no parent")
	}
	if srcTarget.Node.Kind() == NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: cannot hardlink a directory")
	}
	dstTarget, _, err := v.walkEntry(ctx, startDrive, linkPath, false)
	if err != nil {
		return err
	}
	if dstTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link path has no parent")
	}
	return v.nodeOp.Link(ctx, srcTarget, dstTarget.Parent, dstTarget.Name)
}
