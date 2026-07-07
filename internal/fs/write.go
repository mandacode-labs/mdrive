package fs

import (
	"context"

	"github.com/oklog/ulid/v2"
)

// Write replaces the inline data of a file-kind node.
// Mirrors write(2) (stateless). ActionEdit on the resolved
// drive.
func (f *fs) Write(ctx context.Context, driveID, path string, data []byte) error {
	return f.doWrite(ctx, driveID, path, data)
}

func (f *fs) doWrite(ctx context.Context, driveID, path string, data []byte) error {
	id, err := ulid.Parse(driveID)
	if err != nil {
		return err
	}
	dentry, err := f.vfs.Lookup(ctx, id, path, true)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, dentry.DriveID); err != nil {
		return err
	}
	return f.vfs.Write(ctx, dentry, data)
}
