package fs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Read returns the inline data of a file-kind node. Mirrors
// read(2) (stateless). ActionView on the resolved drive.
func (f *fs) Read(ctx context.Context, driveID, path string) ([]byte, error) {
	return f.doRead(ctx, driveID, path)
}

func (f *fs) doRead(ctx context.Context, driveID, path string) ([]byte, error) {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: invalid drive id")
	}
	dentry, err := f.vfs.Lookup(ctx, id, path, true)
	if err != nil {
		return nil, err
	}
	if err := f.requireView(ctx, dentry.DriveID); err != nil {
		return nil, err
	}
	return f.vfs.Read(ctx, dentry)
}
