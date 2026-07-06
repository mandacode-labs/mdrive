package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Read returns the inline data of a file-kind node. Object-kind
// nodes return an error (use upload.PresignDownload for those).
// Linux vfs_read.
func (v *vfs) Read(ctx context.Context, driveID string, path string) ([]byte, error) {
	dentry, err := v.resolveTarget(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return nil, err
	}
	if dentry.Node.Kind() != NodeKindFile {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a file")
	}
	return dentry.Node.Data(), nil
}

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
	// Not found → create.
	_, err := v.Create(ctx, driveID, path, NodeKindFile, data)
	return err
}
