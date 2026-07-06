package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

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