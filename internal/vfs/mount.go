package vfs

import (
	"context"
	"encoding/json"
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
	if _, err := v.resolveTarget(ctx, driveID, mountPath, permission.ActionView); err == nil {
		return errorx.New(errorx.KindAlreadyExists, "vfs: mount path already exists")
	}
	srcULID, err := ulid.Parse(sourceDriveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid source drive id", errorx.KindInvalidArgument)
	}
	if _, err := v.superop.GetRootNodeID(ctx, srcULID); err != nil {
		return err
	}
	target, err := v.resolveTarget(ctx, driveID, mountPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	if target.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount path has no parent")
	}
	if target.Parent.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount parent is not a directory")
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