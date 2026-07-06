package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// ReadObject returns the S3 metadata of an Object-kind node.
// Caller hands it to upload.PresignDownload.
func (v *vfs) ReadObject(ctx context.Context, dentry *Dentry) (ObjectRef, error) {
	if dentry == nil || dentry.Node == nil {
		return ObjectRef{}, errorx.New(errorx.KindInvalidArgument, "vfs: readobject requires a dentry")
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
