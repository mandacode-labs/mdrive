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

// Create creates an empty inode. Linux vfs_create + vfs_mkdir +
// vfs_mknod, unified via kind. No kind-specific data is set
// here; that is the job of the kind-specific command
// (Write / WriteObject / Mount / Symlink).
//
// A directory-kind inode is initialized with an empty
// DirContent so subsequent Lookup/Mknod on it succeeds.
func (v *vfs) Create(ctx context.Context, driveID string, path string, kind NodeKind) (*Node, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	target, err := v.walk(ctx, startDrive, path, false)
	if err != nil {
		return nil, err
	}
	if target.Parent == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: path has no parent")
	}
	if target.Parent.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: parent is not a directory")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return nil, err
	}

	now := time.Now()
	child := NewNode(uuid.New(), startDrive, kind)
	child.atime = now
	child.mtime = now
	child.ctime = now
	child.btime = now

	if kind == NodeKindDirectory {
		empty := &content.DirContent{}
		data, err := empty.Marshal()
		if err != nil {
			return nil, errorx.Wrap(err, "vfs: failed to marshal empty dir content")
		}
		if err := child.Write(data, int64(len(data))); err != nil {
			return nil, err
		}
	}

	if err := v.nodeOp.Create(ctx, target.Parent, child, target.Name); err != nil {
		return nil, err
	}
	return child, nil
}
