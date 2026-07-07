package vfs

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Walk — Linux link_path_walk. Resolves a path string into
// a Dentry, following mount points and (optionally) trailing
// symlinks.
func (v *vfs) Walk(ctx context.Context, driveID ulid.ULID, path string, follow bool) (*fs.Dentry, error) {
	sb, err := v.superop.GetByDriveID(ctx, driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "fs: superblock", errorx.KindInternal)
	}
	root, err := v.nodeOp.Get(ctx, sb.RootNodeID())
	if err != nil {
		return nil, errorx.Wrap(err, "fs: root", errorx.KindInternal)
	}
	if root.Kind() != fs.NodeKindDirectory {
		return nil, errorx.New(errorx.KindInternal, "fs: drive root is not a directory")
	}
	cur := fs.NewRootDentry(sb.DriveID(), root)
	for _, name := range splitPath(path) {
		next, err := v.WalkOne(ctx, cur, name)
		if err != nil {
			return nil, err
		}
		cur = next
	}
	if follow && cur.Node.Kind() == fs.NodeKindSymlink {
		return v.FollowSymlink(ctx, cur, 8)
	}
	return cur, nil
}

// WalkOne — Linux lookup_one.
func (v *vfs) WalkOne(ctx context.Context, cur *fs.Dentry, name string) (*fs.Dentry, error) {
	if name == "" || name == "." {
		return cur, nil
	}
	if name == ".." {
		if cur.Parent != nil {
			return cur.Parent, nil
		}
		return cur, nil
	}
	if cur.Node.Kind() != fs.NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: walk over non-directory")
	}
	var dc fs.DirContent
	if err := json.Unmarshal(cur.Node.Data(), &dc); err != nil {
		return nil, errorx.Wrap(err, "fs: dir content", errorx.KindInternal)
	}
	entry := dc.FindEntry(name)
	if entry == nil {
		return nil, errorx.New(errorx.KindNotFound, "fs: not found")
	}
	child, err := v.nodeOp.Get(ctx, entry.NodeID)
	if err != nil {
		return nil, errorx.Wrap(err, "fs: inode", errorx.KindInternal)
	}
	if child.Kind() == fs.NodeKindMount {
		return v.FollowMount(ctx, cur, child)
	}
	return &fs.Dentry{DriveID: cur.DriveID, Parent: cur, Name: name, Node: child}, nil
}

// FollowMount — Linux <fs>_follow_link for mounts.
func (v *vfs) FollowMount(ctx context.Context, parent *fs.Dentry, mount *fs.Node) (*fs.Dentry, error) {
	var mc fs.MountContent
	if err := json.Unmarshal(mount.Data(), &mc); err != nil {
		return nil, errorx.Wrap(err, "fs: mount content", errorx.KindInternal)
	}
	if mc.DriveID == "" {
		return nil, errorx.New(errorx.KindInternal, "fs: mount without source drive id")
	}
	srcULID, err := ulid.Parse(mc.DriveID)
	if err != nil {
		return nil, errorx.Wrap(err, "fs: invalid mount source drive id", errorx.KindInternal)
	}
	srcSB, err := v.superop.GetByDriveID(ctx, srcULID)
	if err != nil {
		return nil, errorx.Wrap(err, "fs: source superblock", errorx.KindInternal)
	}
	srcRoot, err := v.nodeOp.Get(ctx, srcSB.RootNodeID())
	if err != nil {
		return nil, errorx.Wrap(err, "fs: source root", errorx.KindInternal)
	}
	return fs.NewMountRootDentry(srcSB.DriveID(), parent, srcRoot), nil
}

// FollowSymlink — Linux <fs>_follow_link for symlinks.
func (v *vfs) FollowSymlink(ctx context.Context, cur *fs.Dentry, depth int) (*fs.Dentry, error) {
	if depth == 0 {
		return nil, errorx.New(errorx.KindFailedPrecondition, "fs: symlink loop")
	}
	if cur.Node.Kind() != fs.NodeKindSymlink {
		return cur, nil
	}
	var sc fs.SymlinkContent
	if err := json.Unmarshal(cur.Node.Data(), &sc); err != nil {
		return nil, errorx.Wrap(err, "fs: symlink content", errorx.KindInternal)
	}
	target, err := v.nodeOp.Get(ctx, sc.NodeID)
	if err != nil {
		return nil, errorx.Wrap(err, "fs: symlink target", errorx.KindInternal)
	}
	return v.FollowSymlink(ctx, &fs.Dentry{
		DriveID: cur.DriveID, Parent: cur.Parent, Name: cur.Name, Node: target,
	}, depth-1)
}

// splitPath splits an absolute path into components.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	if p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
