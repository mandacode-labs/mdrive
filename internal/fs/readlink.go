package fs

import "context"

// ReadlinkAt returns the symlink target's inode id as a
// string. Mirrors readlinkat(2). ActionView on the resolved
// drive.
func (f *fs) ReadlinkAt(ctx context.Context, driveID, path string) (string, error) {
	return f.doReadlink(ctx, driveID, path)
}

func (f *fs) doReadlink(ctx context.Context, driveID, path string) (string, error) {
	linkParent, linkName, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return "", err
	}
	if err := f.requireView(ctx, linkParent.DriveID); err != nil {
		return "", err
	}
	dentry, err := f.vfs.WalkOne(ctx, linkParent, linkName)
	if err != nil {
		return "", err
	}
	targetID, err := f.vfs.Readlink(ctx, dentry)
	if err != nil {
		return "", err
	}
	return targetID.String(), nil
}
