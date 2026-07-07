package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// mount — Linux bind-mount.
func (v *vfs) mount(ctx context.Context, mountParent *fs.Dentry, mountName string, sourceDriveID ulid.ULID) error {
	if mountParent == nil || mountName == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: mount requires parent and name")
	}
	if mountParent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: mount parent is not a directory")
	}
	if _, err := v.superop.GetByDriveID(ctx, sourceDriveID); err != nil {
		return errorx.Wrap(err, "fs: source drive not found", errorx.KindInvalidArgument)
	}
	now := time.Now()
	mount := fs.NewNode(uuid.New(), mountParent.Node.SuperblockID(), fs.NodeKindMount)
	mount.atime = now
	mount.mtime = now
	mount.ctime = now
	mount.btime = now
	mc := &content.MountContent{DriveID: sourceDriveID.String()}
	data, err := mc.Marshal()
	if err != nil {
		return errorx.Wrap(err, "fs: mount content", errorx.KindInternal)
	}
	if err := mount.Write(data, int64(len(data))); err != nil {
		return err
	}
	return v.nodeOp.Create(ctx, mountParent.Node, mount, mountName)
}
