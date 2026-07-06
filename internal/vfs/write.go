package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Write creates or overwrites a file-kind node with the given
// inline data. Files larger than MaxDataSize return
// KindInvalidArgument. Linux vfs_write.
func (v *vfs) Write(ctx context.Context, driveID string, path string, data []byte) error {
	if _, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit); err == nil {
		dentry, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit)
		if err != nil {
			return err
		}
		if dentry.Node.Kind() != NodeKindFile {
			return errorx.New(errorx.KindInvalidArgument, "vfs: target exists and is not a file")
		}
		return dentry.Node.Write(data, int64(len(data)))
	}
	// Not found → create the file with data.
	_, err := v.createWithData(ctx, driveID, path, NodeKindFile, data)
	return err
}

// createWithData is the shared path for fresh Create + data.
func (v *vfs) createWithData(ctx context.Context, driveID string, path string, kind NodeKind, data []byte) (*Node, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	target, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit)
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
	if err := child.Write(data, int64(len(data))); err != nil {
		return nil, err
	}
	if err := v.nodeOp.Create(ctx, target.Parent, child, target.Name); err != nil {
		return nil, err
	}
	return child, nil
}