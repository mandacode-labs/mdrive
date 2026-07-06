package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Mount installs a mount point at mountPath that resolves into
// sourceDriveID's root. The mount-kind node's inline content is
// a content.MountContent. Linux vfs_mount.
//
// Source drive existence is verified via the superblock.
func (v *vfs) Mount(ctx context.Context, driveID string, mountPath string, sourceDriveID string) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	if _, err := v.walk(ctx, startDrive, mountPath, false); err == nil {
		return errorx.New(errorx.KindAlreadyExists, "vfs: mount path already exists")
	}
	srcULID, err := ulid.Parse(sourceDriveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid source drive id", errorx.KindInvalidArgument)
	}
	if _, err := v.superop.GetByDriveID(ctx, srcULID); err != nil {
		return err
	}
	target, err := v.walk(ctx, startDrive, mountPath, false)
	if err != nil {
		return err
	}
	if target.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount path has no parent")
	}
	if target.Parent.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount parent is not a directory")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return err
	}

	now := time.Now()
	mount := NewNode(uuid.New(), target.Parent.Drive(), NodeKindMount)
	mount.atime = now
	mount.mtime = now
	mount.ctime = now
	mount.btime = now
	mc := &content.MountContent{DriveID: sourceDriveID}
	mcData, err := mc.Marshal()
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal mount content")
	}
	if err := mount.Write(mcData, int64(len(mcData))); err != nil {
		return err
	}
	return v.nodeOp.Create(ctx, target.Parent, mount, target.Name)
}
