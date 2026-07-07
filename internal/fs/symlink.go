package fs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// SymlinkAt creates a symbolic link. Mirrors symlinkat(2).
// `target` is resolved to its node id; the symlink node
// stores it as SymlinkContent. ActionEdit on link's parent
// drive.
func (f *fs) SymlinkAt(ctx context.Context, driveID, target, linkPath string) (Stat, error) {
	linkParent, linkName, err := f.doPathParent(ctx, driveID, linkPath)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, linkParent.DriveID); err != nil {
		return Stat{}, err
	}
	targetDentry, err := f.walkResolve(ctx, linkParent.DriveID.String(), target)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireView(ctx, targetDentry.DriveID); err != nil {
		return Stat{}, err
	}
	link, err := f.vfs.Symlink(ctx, linkParent, linkName, targetDentry.Node.ID())
	if err != nil {
		return Stat{}, err
	}
	return NodeToStat(link), nil
}

// ReadlinkAt returns the symlink target's stored content.
// The caller can inspect TargetID and follow it via
// Stat(driveID, targetID, true) if needed.
// ActionView on the link's parent drive.
func (f *fs) ReadlinkAt(ctx context.Context, driveID, path string) (*content.SymlinkContent, error) {
	linkParent, linkName, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return nil, err
	}
	if err := f.requireView(ctx, linkParent.DriveID); err != nil {
		return nil, err
	}
	dentry, err := f.vfs.WalkOne(ctx, linkParent, linkName)
	if err != nil {
		return nil, err
	}
	if dentry.Node.Kind() != NodeKindSymlink {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: not a symlink")
	}
	var sc content.SymlinkContent
	if err := json.Unmarshal(dentry.Node.Data(), &sc); err != nil {
		return nil, errorx.Wrap(err, "fs: symlink content", errorx.KindInternal)
	}
	return &sc, nil
}