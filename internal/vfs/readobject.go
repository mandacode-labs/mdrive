package vfs

import (
	"context"
	"encoding/json"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// ReadObject retrieves the S3 metadata stored in an Object-kind
// inode. Linux vfs_read on an object backend has the same shape:
// walk to the inode, hand back a handle for the caller to issue
// the actual GET. mdrive returns the ObjectRef; the caller
// (handler, upload service) is responsible for translating it
// into a presigned download URL.
//
// Symlinks along the path are followed (stat semantics). The
// final node must be of Object kind.
func (v *vfs) ReadObject(ctx context.Context, driveID string, path string) (ObjectRef, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return ObjectRef{}, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, err := v.walk(ctx, startDrive, path, true)
	if err != nil {
		return ObjectRef{}, err
	}
	if dentry.Node.Kind() != NodeKindObject {
		return ObjectRef{}, errorx.New(errorx.KindInvalidArgument, "vfs: not an object")
	}
	var oc content.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return ObjectRef{}, errorx.Wrap(err, "vfs: invalid object content", errorx.KindInternal)
	}
	return ObjectRef{
		Bucket:   oc.Bucket,
		Key:      oc.Key,
		Mime:     oc.Mime,
		Checksum: oc.Checksum,
	}, nil
}
