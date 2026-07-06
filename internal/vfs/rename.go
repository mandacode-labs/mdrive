package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Rename moves an entry from srcDriveID/srcPath to
// dstDriveID/dstPath. Cross-drive rename is not supported;
// src and dst drives must match. Linux vfs_rename.
func (v *vfs) Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error {
	if srcDriveID != dstDriveID {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: cross-drive rename not supported")
	}
	srcTarget, err := v.resolveTarget(ctx, srcDriveID, srcPath, permission.ActionView)
	if err != nil {
		return err
	}
	if srcTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: source path has no parent")
	}
	dstTarget, err := v.resolveTarget(ctx, dstDriveID, dstPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	if dstTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: destination path has no parent")
	}
	return v.nodeOp.Rename(ctx, srcTarget, dstTarget.Parent, dstTarget.Name)
}