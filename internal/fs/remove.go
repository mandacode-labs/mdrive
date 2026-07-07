package fs

import "context"

// Remove is mdrive's `rm -rf` equivalent. Cascades
// vfs.RemoveRecursive for non-empty trees. ActionEdit on
// the parent drive.
//
// Not a single Linux syscall — corresponds to userspace
// `rm -rf`, which loops unlinkat(2) over the tree.
func (f *fs) Remove(ctx context.Context, driveID string, paths []string, opts RemoveOpts) error {
	for _, p := range paths {
		if err := f.doRemove(ctx, driveID, p, opts); err != nil {
			return err
		}
	}
	return nil
}

func (f *fs) doRemove(ctx context.Context, driveID, path string, opts RemoveOpts) error {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, parent.DriveID); err != nil {
		return err
	}
	dentry, err := f.vfs.WalkOne(ctx, parent, name)
	if err != nil {
		return err
	}
	if !opts.Recursive {
		if dentry.Node.Kind() == NodeKindDirectory {
			return f.vfs.Rmdir(ctx, parent, name)
		}
		return f.vfs.Unlink(ctx, parent, name)
	}
	if err := f.vfs.RemoveRecursive(ctx, dentry); err != nil {
		return err
	}
	if dentry.Node.Kind() == NodeKindDirectory {
		return f.vfs.Rmdir(ctx, parent, name)
	}
	return f.vfs.Unlink(ctx, parent, name)
}