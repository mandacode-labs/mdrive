package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Mount installs a mount-point at `mountName` under
// `mountParent`. The mount's source drive is verified via its
// superblock before insertion.
func (v *vfs) Mount(ctx context.Context, mountParent *Dentry, mountName string, sourceDriveID ulid.ULID) error {
	if mountParent == nil || mountName == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount requires parent and name")
	}
	if mountParent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount parent is not a directory")
	}
	if _, err := v.superop.GetByDriveID(ctx, sourceDriveID); err != nil {
		return errorx.Wrap(err, "vfs: source drive not found", errorx.KindInvalidArgument)
	}

	now := time.Now()
	mount := NewNode(uuid.New(), mountParent.Node.SuperblockID(), NodeKindMount)
	mount.atime = now
	mount.mtime = now
	mount.ctime = now
	mount.btime = now
	mc := &content.MountContent{DriveID: sourceDriveID.String()}
	data, err := mc.Marshal()
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal mount content", errorx.KindInternal)
	}
	if err := mount.Write(data, int64(len(data))); err != nil {
		return err
	}
	return v.nodeOp.Create(ctx, mountParent.Node, mount, mountName)
}
