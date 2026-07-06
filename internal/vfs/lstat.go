package vfs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Lstat returns the symlink itself without following it.
// Linux lstat(2).
func (v *vfs) Lstat(ctx context.Context, driveID string, path string) (*Node, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, _, err := v.walkEntry(ctx, startDrive, path, false)
	if err != nil {
		return nil, err
	}
	return dentry.Node, nil
}
