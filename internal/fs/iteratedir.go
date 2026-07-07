package fs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Getdents lists the entries of a directory. Mirrors
// getdents64. ActionView on the resolved drive.
func (f *fs) Getdents(ctx context.Context, driveID, path string) ([]DirEntry, error) {
	return f.doGetdents(ctx, driveID, path)
}

func (f *fs) doGetdents(ctx context.Context, driveID, path string) ([]DirEntry, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	dentry, err := f.vfs.lookup(ctx, id, path, true)
	if err != nil {
		return nil, err
	}
	if err := f.requireView(ctx, dentry.DriveID); err != nil {
		return nil, err
	}
	return f.vfs.iterate(ctx, dentry)
}
