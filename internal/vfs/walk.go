package vfs

import (
	"context"
	"encoding/json"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// walkStep walks one component. Mount nodes cross to the source
// drive (with an ActionView check); symlinks are passed through
// for walk to resolve at the end.
func (v *vfs) walkStep(ctx context.Context, cur *Dentry, name string) (*Dentry, error) {
	if name == "" || name == "." {
		return cur, nil
	}
	if cur.Node.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: path component over non-directory")
	}

	var dirContent content.DirContent
	if err := json.Unmarshal(cur.Node.Data(), &dirContent); err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to unmarshal directory content", errorx.KindInternal)
	}
	entry := dirContent.FindEntry(name)
	if entry == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: path component not found")
	}

	child, err := v.nodeOp.Get(ctx, entry.NodeID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to read inode", errorx.KindInternal)
	}

	if child.Kind() == NodeKindMount {
		return v.walkMount(ctx, child)
	}
	return &Dentry{Parent: cur.Node, Name: name, Node: child}, nil
}

// walkMount resolves a mount node to the source drive's root
// after an ActionView check on the source drive.
func (v *vfs) walkMount(ctx context.Context, mount *Node) (*Dentry, error) {
	var mc content.MountContent
	if err := json.Unmarshal(mount.Data(), &mc); err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid mount content", errorx.KindInternal)
	}
	if mc.DriveID == "" {
		return nil, errorx.New(errorx.KindInternal, "vfs: mount without source drive id")
	}
	srcULID, err := ulid.Parse(mc.DriveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid mount source drive id", errorx.KindInternal)
	}
	if err := v.checkPerm(ctx, permission.ActionView, srcULID); err != nil {
		return nil, err
	}
	srcSB, err := v.superop.GetByDriveID(ctx, srcULID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to get source superblock", errorx.KindInternal)
	}
	srcRoot, err := v.nodeOp.Get(ctx, srcSB.RootNodeID())
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to get source root", errorx.KindInternal)
	}
	return &Dentry{Parent: srcRoot, Name: "/", Node: srcRoot}, nil
}

// followSymlink resolves a symlink chain, depth-capped to break loops.
func (v *vfs) followSymlink(ctx context.Context, cur *Dentry, depth int) (*Dentry, error) {
	if depth == 0 {
		return nil, errorx.New(errorx.KindFailedPrecondition, "vfs: symlink loop")
	}
	if cur.Node.Kind() != NodeKindSymlink {
		return cur, nil
	}
	var sc content.SymlinkContent
	if err := json.Unmarshal(cur.Node.Data(), &sc); err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid symlink content", errorx.KindInternal)
	}
	target, err := v.nodeOp.Get(ctx, sc.NodeID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to read symlink target", errorx.KindInternal)
	}
	resolved := &Dentry{Parent: cur.Parent, Name: cur.Name, Node: target}
	return v.followSymlink(ctx, resolved, depth-1)
}

// splitPath splits an absolute path into components. Empty
// components and trailing slashes are dropped.
func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, c := range p {
		if c == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
