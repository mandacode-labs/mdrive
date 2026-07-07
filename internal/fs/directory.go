package fs

import "context"

// Mkdir creates a directory. Mirrors mkdir(2).
// ActionEdit on the parent drive.
func (f *fs) Mkdir(ctx context.Context, driveID, path string) (Stat, error) {
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, parent.DriveID); err != nil {
		return Stat{}, err
	}
	node, err := f.vfs.Mkdir(ctx, parent, name)
	if err != nil {
		return Stat{}, err
	}
	return NodeToStat(node), nil
}

// Getdents lists the entries of a directory. Mirrors
// getdents64. ActionView on the resolved drive.
func (f *fs) Getdents(ctx context.Context, driveID, path string) (*DirContent, error) {
	dentry, err := f.walkForKind(ctx, driveID, path, NodeKindDirectory)
	if err != nil {
		return nil, err
	}
	if err := f.requireView(ctx, dentry.DriveID); err != nil {
		return nil, err
	}
	entries, err := f.vfs.Iterate(ctx, dentry)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []DirEntry{}
	}
	return &DirContent{Entries: entries}, nil
}
