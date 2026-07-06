package vfs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Stat follows the trailing symlink. Linux stat(2).
func (v *vfs) Stat(ctx context.Context, driveID string, path string) (*Node, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, _, err := v.walkEntry(ctx, startDrive, path, true)
	if err != nil {
		return nil, err
	}
	return dentry.Node, nil
}
