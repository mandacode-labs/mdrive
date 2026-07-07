package vfs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/provider"
)

// vfs is the concrete implementation of fs.VFS.
type vfs struct {
	nodeOp    fs.NodeOperation
	superop   fs.SuperOperation
	provider  provider.StorageProvider
}

// Config groups the dependencies of vfs.
type Config struct {
	NodeOp   fs.NodeOperation
	SuperOp  fs.SuperOperation
	Provider provider.StorageProvider
}

// New constructs an fs.VFS implementation.
func New(cfg Config) fs.VFS {
	return &vfs{
		nodeOp:   cfg.NodeOp,
		superop:  cfg.SuperOp,
		provider: cfg.Provider,
	}
}

// Download returns a presigned GET URL for an object-kind node.
func (v *vfs) Download(ctx context.Context, dentry *fs.Dentry, expiry time.Duration) (string, error) {
	if dentry.Node.Kind() != fs.NodeKindObject {
		return "", errorx.New(errorx.KindInvalidArgument, "vfs: download target is not an object")
	}
	var oc fs.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return "", errorx.Wrap(err, "vfs: object content", errorx.KindInternal)
	}
	return v.provider.PresignDownload(ctx, oc.Key, expiry)
}

// Upload returns a presigned PUT URL for a new object at
// parent/name. The caller PUTs to URL and then calls
// Service.Complete with the returned Key to finalize the node.
func (v *vfs) Upload(ctx context.Context, parent *fs.Dentry, key, contentType string, expiry time.Duration) (provider.UploadInfo, error) {
	return v.provider.PresignUpload(ctx, key, contentType, expiry)
}

// Verify returns the backend-reported metadata for an
// existing object-kind node.
func (v *vfs) Verify(ctx context.Context, dentry *fs.Dentry) (provider.ObjectMetadata, error) {
	if dentry.Node.Kind() != fs.NodeKindObject {
		return provider.ObjectMetadata{}, errorx.New(errorx.KindInvalidArgument, "vfs: verify target is not an object")
	}
	var oc fs.ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return provider.ObjectMetadata{}, errorx.Wrap(err, "vfs: object content", errorx.KindInternal)
	}
	return v.provider.HeadObject(ctx, oc.Key)
}

// VerifyByKey returns backend metadata for a key. Used by
// Service.Complete (the node doesn't exist yet).
func (v *vfs) VerifyByKey(ctx context.Context, key string) (provider.ObjectMetadata, error) {
	return v.provider.HeadObject(ctx, key)
}