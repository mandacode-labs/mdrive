package vfs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Read returns the inline data of a file-kind node. Object-kind
// reads use ReadObject. Linux vfs_read.
func (v *vfs) Read(ctx context.Context, driveID string, path string) ([]byte, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, _, err := v.walkEntry(ctx, startDrive, path, true)
	if err != nil {
		return nil, err
	}
	if dentry.Node.Kind() != NodeKindFile {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a file")
	}
	return dentry.Node.Data(), nil
}
