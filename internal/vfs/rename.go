package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Rename moves an entry from srcDriveID/srcPath to
// dstDriveID/dstPath. Cross-drive rename is not supported in
// phase 2; src and dst drives must match. Linux vfs_rename.
func (v *vfs) Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error {
	if srcDriveID != dstDriveID {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: cross-drive rename not supported")
	}
	if _, err := v.resolveTarget(ctx, srcDriveID, srcPath, permission.ActionView); err != nil {
		return err
	}
	srcParent, srcName, err := v.resolveParent(ctx, srcDriveID, srcPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	dstParent, dstName, err := v.resolveParent(ctx, dstDriveID, dstPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	srcDentry, err := v.nodeOp.Lookup(ctx, srcParent.Node, srcName)
	if err != nil {
		return err
	}
	return v.nodeOp.Rename(ctx, srcDentry, dstParent.Node, dstName)
}

// Link creates a hardlink: dstDriveID/dstPath becomes a second
// directory entry pointing at the same inode as srcPath. Linux
// vfs_link.
func (v *vfs) Link(ctx context.Context, driveID string, srcPath string, linkPath string) error {
	if _, err := v.resolveTarget(ctx, driveID, srcPath, permission.ActionView); err != nil {
		return err
	}
	srcParent, srcName, err := v.resolveParent(ctx, driveID, srcPath, permission.ActionView)
	if err != nil {
		return err
	}
	srcDentry, err := v.nodeOp.Lookup(ctx, srcParent.Node, srcName)
	if err != nil {
		return err
	}
	if srcDentry.Node.Kind() == NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: cannot hardlink a directory")
	}
	dstParent, dstName, err := v.resolveParent(ctx, driveID, linkPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	return v.nodeOp.Link(ctx, srcDentry, dstParent.Node, dstName)
}
