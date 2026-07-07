package fs

import "context"

// LinkAt creates a hard link. Mirrors linkat(2). Refuses
// directories. ActionEdit on both source and destination
// parents.
func (f *fs) LinkAt(ctx context.Context, driveID, srcPath, linkPath string) (Stat, error) {
	return f.doLink(ctx, driveID, srcPath, linkPath)
}

func (f *fs) doLink(ctx context.Context, driveID, srcPath, linkPath string) (Stat, error) {
	srcParent, srcName, err := f.doPathParent(ctx, driveID, srcPath)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, srcParent.DriveID); err != nil {
		return Stat{}, err
	}
	linkParent, linkName, err := f.doPathParent(ctx, driveID, linkPath)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, linkParent.DriveID); err != nil {
		return Stat{}, err
	}
	srcDentry, err := f.vfs.walkOne(ctx, srcParent, srcName)
	if err != nil {
		return Stat{}, err
	}
	if err := f.vfs.link(ctx, srcDentry, linkParent, linkName); err != nil {
		return Stat{}, err
	}
	return NodeToStat(srcDentry.Node), nil
}

// SymlinkAt creates a symbolic link. Mirrors symlinkat(2).
// `target` is resolved to its node id (graph-based content).
// ActionEdit on the link's parent drive.
func (f *fs) SymlinkAt(ctx context.Context, driveID, target, linkPath string) (Stat, error) {
	return f.doSymlink(ctx, driveID, target, linkPath)
}

func (f *fs) doSymlink(ctx context.Context, driveID, target, linkPath string) (Stat, error) {
	linkParent, linkName, err := f.doPathParent(ctx, driveID, linkPath)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, linkParent.DriveID); err != nil {
		return Stat{}, err
	}
	targetDentry, err := f.vfs.lookup(ctx, linkParent.DriveID, target, true)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireView(ctx, targetDentry.DriveID); err != nil {
		return Stat{}, err
	}
	link, err := f.vfs.symlink(ctx, linkParent, linkName, targetDentry.Node.ID())
	if err != nil {
		return Stat{}, err
	}
	return NodeToStat(link), nil
}

// Unlink removes a non-directory entry. Mirrors unlink(2).
// ActionEdit on the parent drive.
func (f *fs) Unlink(ctx context.Context, driveID, path string) error {
	return f.doUnlink(ctx, driveID, path)
}

func (f *fs) doUnlink(ctx context.Context, driveID, path string) error {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, parent.DriveID); err != nil {
		return err
	}
	return f.vfs.unlink(ctx, parent, name)
}
