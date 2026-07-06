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

// WriteObject creates or replaces an Object-kind node. vfs
// stores the ref inline as content.ObjectContent. Caller
// (typically the handler after a successful S3 PUT) supplies
// the bucket, key, mime type, and optional checksum.
func (v *vfs) WriteObject(ctx context.Context, driveID string, path string, ref ObjectRef) error {
	if ref.Bucket == "" || ref.Key == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: object ref requires bucket and key")
	}

	oc := &content.ObjectContent{
		Bucket:   ref.Bucket,
		Key:      ref.Key,
		Mime:     ref.Mime,
		Checksum: ref.Checksum,
	}
	data, err := oc.Marshal()
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal object content")
	}

	// Try replace first, else create.
	if _, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit); err == nil {
		target, err := v.resolveTarget(ctx, driveID, path, permission.ActionEdit)
		if err != nil {
			return err
		}
		if target.Parent == nil {
			return errorx.New(errorx.KindInvalidArgument, "vfs: path has no parent")
		}
		if target.Node.Kind() != NodeKindObject {
			return errorx.New(errorx.KindInvalidArgument, "vfs: target exists and is not an object")
		}
		if err := v.nodeOp.Unlink(ctx, target); err != nil {
			return errorx.Wrap(err, "vfs: failed to remove existing object")
		}
		startDrive, err := ulid.Parse(driveID)
		if err != nil {
			return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
		}
		now := time.Now()
		child := NewNode(uuid.New(), startDrive, NodeKindObject)
		child.atime = now
		child.mtime = now
		child.ctime = now
		child.btime = now
		if err := child.Write(data, int64(len(data))); err != nil {
			return err
		}
		return v.nodeOp.Create(ctx, target.Parent, child, target.Name)
	}
	// Not found → create fresh.
	_, err = v.createWithData(ctx, driveID, path, NodeKindObject, data)
	return err
}