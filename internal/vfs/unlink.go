package vfs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Unlink removes a non-directory entry. nlink==0 destroys the
// inode. Linux vfs_unlink. Permission: ActionEdit on the drive.
func (v *vfs) Unlink(ctx context.Context, driveID string, path string) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, err := v.walk(ctx, startDrive, path, false)
	if err != nil {
		return err
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return err
	}
	if dentry.Node.Kind() == NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: cannot unlink a directory; use Rmdir")
	}
	if dentry.Parent == nil {
		return errorx.New(errorx.KindInternal, "vfs: dentry has no parent")
	}
	if dentry.Parent.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInternal, "vfs: parent is not a directory")
	}
	return v.nodeOp.Unlink(ctx, dentry)
}
