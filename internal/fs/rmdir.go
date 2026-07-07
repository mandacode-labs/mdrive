package fs

import "context"

// Rmdir removes an empty directory. Mirrors rmdir(2).
// ActionEdit on the parent drive.
func (f *fs) Rmdir(ctx context.Context, driveID, path string) error {
	return f.doRmdir(ctx, driveID, path)
}

func (f *fs) doRmdir(ctx context.Context, driveID, path string) error {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, parent.DriveID); err != nil {
		return err
	}
	return f.vfs.Rmdir(ctx, parent, name)
}
