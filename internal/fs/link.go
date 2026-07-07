package fs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// LinkAt creates a hard link. Mirrors linkat(2). Refuses
// directories. ActionEdit on source and destination.
func (f *fs) LinkAt(ctx context.Context, driveID, srcPath, linkPath string) (Stat, error) {
	srcParent, srcName, err := f.doPathParent(ctx, driveID, srcPath)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireWrite(ctx, srcParent.DriveID); err != nil {
		return Stat{}, err
	}
	linkParent, linkName, err := f.doPathParent(ctx, driveID, linkPath)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireWrite(ctx, linkParent.DriveID); err != nil {
		return Stat{}, err
	}
	srcDentry, err := f.vfs.WalkOne(ctx, srcParent, srcName)
	if err != nil {
		return Stat{}, err
	}
	if srcDentry.Node.Kind() == NodeKindDirectory {
		return Stat{}, errorx.New(errorx.KindInvalidArgument, "fs: cannot hardlink a directory")
	}
	if err := f.vfs.Link(ctx, srcDentry, linkParent, linkName); err != nil {
		return Stat{}, err
	}
	return NodeToStat(srcDentry.Node), nil
}

// Unlink removes a non-directory entry. Mirrors unlink(2).
// ActionDelete.
func (f *fs) Unlink(ctx context.Context, driveID, path string) error {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return err
	}
	if err := f.requireDelete(ctx, parent.DriveID); err != nil {
		return err
	}
	return f.vfs.Unlink(ctx, parent, name)
}

// Rmdir removes an empty directory. Mirrors rmdir(2).
// ActionDelete.
func (f *fs) Rmdir(ctx context.Context, driveID, path string) error {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return err
	}
	if err := f.requireDelete(ctx, parent.DriveID); err != nil {
		return err
	}
	return f.vfs.Rmdir(ctx, parent, name)
}

// RenameAt moves a single entry. Mirrors renameat(2).
// Cross-drive rename is rejected (parents must share a
// superblock). ActionEdit on source and destination.
func (f *fs) RenameAt(ctx context.Context, driveID, srcPath, dstDriveID, dstPath string) error {
	srcParent, srcName, err := f.doPathParent(ctx, driveID, srcPath)
	if err != nil {
		return err
	}
	if err := f.requireWrite(ctx, srcParent.DriveID); err != nil {
		return err
	}
	dstParent, dstName, err := f.doPathParent(ctx, dstDriveID, dstPath)
	if err != nil {
		return err
	}
	if err := f.requireWrite(ctx, dstParent.DriveID); err != nil {
		return err
	}
	return f.vfs.Rename(ctx, srcParent, srcName, dstParent, dstName)
}
