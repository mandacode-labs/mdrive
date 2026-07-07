package vfs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Download returns a presigned GET URL for an object-kind node.
func (v *vfs) Download(ctx context.Context, dentry *fs.Dentry, expiry time.Duration) (string, error) {
	if dentry.Node.Kind() != fs.NodeKindObject {
		return "", errorx.New(errorx.KindInvalidArgument, "vfs: download target is not an object")
	}
	var oc fs.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return "", errorx.Wrap(err, "vfs: object content", errorx.KindInternal)
	}
	p, err := v.resolvePresigner(ctx, dentry.Node.SuperblockID())
	if err != nil {
		return "", err
	}
	return p.PresignDownload(ctx, oc.Key, expiry)
}

// Upload returns a presigned PUT URL for a future object at
// parent/name. The caller PUTs to URL and then calls
// Service.Complete with the returned Key to finalize the node.
func (v *vfs) Upload(ctx context.Context, parent *fs.Dentry, key, contentType string, expiry time.Duration) (fs.UploadInfo, error) {
	p, err := v.resolvePresigner(ctx, parent.Node.SuperblockID())
	if err != nil {
		return fs.UploadInfo{}, err
	}
	return p.PresignUpload(ctx, key, contentType, expiry)
}

// Verify returns the backend-reported metadata for an
// existing object-kind node.
func (v *vfs) Verify(ctx context.Context, dentry *fs.Dentry) (fs.ObjectMetadata, error) {
	if dentry.Node.Kind() != fs.NodeKindObject {
		return fs.ObjectMetadata{}, errorx.New(errorx.KindInvalidArgument, "vfs: verify target is not an object")
	}
	var oc fs.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return fs.ObjectMetadata{}, errorx.Wrap(err, "vfs: object content", errorx.KindInternal)
	}
	p, err := v.resolvePresigner(ctx, dentry.Node.SuperblockID())
	if err != nil {
		return fs.ObjectMetadata{}, err
	}
	return p.ObjectMetadata(ctx, oc.Key)
}

// VerifyByKey returns backend metadata for a key under a
// specific superblock. Used by Service.Complete (the node
// doesn't exist yet).
func (v *vfs) VerifyByKey(ctx context.Context, sbID uuid.UUID, key string) (fs.ObjectMetadata, error) {
	p, err := v.resolvePresigner(ctx, sbID)
	if err != nil {
		return fs.ObjectMetadata{}, err
	}
	return p.ObjectMetadata(ctx, key)
}