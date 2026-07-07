package fs

import "context"

// RenameAt moves a single entry. Mirrors renameat(2).
// Cross-drive rename is rejected (parents must share a
// superblock, like Linux vfs_rename). ActionEdit on both
// parent drives.
func (f *fs) RenameAt(ctx context.Context, driveID, srcPath, dstDriveID, dstPath string) error {
	return f.doRename(ctx, driveID, srcPath, dstDriveID, dstPath)
}

func (f *fs) doRename(ctx context.Context, driveID, srcPath, dstDriveID, dstPath string) error {
	srcParent, srcName, err := f.doPathParent(ctx, driveID, srcPath)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, srcParent.DriveID); err != nil {
		return err
	}
	dstParent, dstName, err := f.doPathParent(ctx, dstDriveID, dstPath)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, dstParent.DriveID); err != nil {
		return err
	}
	return f.vfs.Rename(ctx, srcParent, srcName, dstParent, dstName)
}
