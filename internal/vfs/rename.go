package vfs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Rename moves an entry. Cross-drive rename is not supported.
// Linux vfs_rename.
func (v *vfs) Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error {
	if srcDriveID != dstDriveID {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: cross-drive rename not supported")
	}
	srcDrive, err := ulid.Parse(srcDriveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid source drive id", errorx.KindInvalidArgument)
	}
	dstDrive, err := ulid.Parse(dstDriveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid destination drive id", errorx.KindInvalidArgument)
	}
	if srcDrive.Compare(dstDrive) != 0 {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: cross-drive rename not supported")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, srcDrive); err != nil {
		return err
	}
	srcTarget, err := v.walk(ctx, srcDrive, srcPath, false)
	if err != nil {
		return err
	}
	if srcTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: source path has no parent")
	}
	dstTarget, err := v.walk(ctx, dstDrive, dstPath, false)
	if err != nil {
		return err
	}
	if dstTarget.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: destination path has no parent")
	}
	return v.nodeOp.Rename(ctx, srcTarget, dstTarget.Parent, dstTarget.Name)
}
