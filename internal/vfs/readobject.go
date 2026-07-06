package vfs

import (
	"context"
	"encoding/json"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// ReadObject returns the S3 metadata of an Object-kind node.
// Caller hands it to upload.PresignDownload. Symlinks are
// followed; final node must be Object kind.
func (v *vfs) ReadObject(ctx context.Context, driveID string, path string) (ObjectRef, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return ObjectRef{}, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, _, err := v.walkEntry(ctx, startDrive, path, true)
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
