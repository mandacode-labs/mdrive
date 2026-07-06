package vfs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Create adds a new entry under `path` inside driveID. The kind
// selects the inode shape: file/directory/symlink/object/mount.
// Permission: ActionEdit on the parent drive.
//
// Linux vfs_create + vfs_mkdir + vfs_mknod (unified via kind).
func (v *vfs) Create(ctx context.Context, driveID string, path string, kind NodeKind, data []byte) (*Node, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	parent, name, err := v.resolveParent(ctx, driveID, path, permission.ActionEdit)
	if err != nil {
		return nil, err
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: parent is not a directory")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return nil, err
	}

	now := time.Now()
	child := NewNode(uuid.New(), startDrive, kind)
	child.atime = now
	child.mtime = now
	child.ctime = now
	child.btime = now
	if data != nil {
		if err := child.Write(data, int64(len(data))); err != nil {
			return nil, err
		}
	}

	// Special-case: directory must be initialized with an empty
	// entry list so subsequent Mknod/Link on it don't fail.
	if kind == NodeKindDirectory && child.Data() == nil {
		if err := child.Write(emptyDirContent(), 0); err != nil {
			return nil, err
		}
	}

	if err := v.nodeOp.Create(ctx, parent.Node, child, name); err != nil {
		return nil, err
	}
	return child, nil
}

// Symlink creates a symlink at linkPath pointing at target.
// Linux vfs_symlink.
func (v *vfs) Symlink(ctx context.Context, driveID string, target string, linkPath string) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	// target resolution needs View perm on its drive
	if _, err := v.resolveTarget(ctx, driveID, target, permission.ActionView); err != nil {
		return err
	}
	parent, name, err := v.resolveParent(ctx, driveID, linkPath, permission.ActionEdit)
	if err != nil {
		return err
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link parent is not a directory")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return err
	}

	// Look up the target so we have its id.
	targetDentry, err := v.resolveTarget(ctx, driveID, target, permission.ActionView)
	if err != nil {
		return err
	}

	now := time.Now()
	link := NewNode(uuid.New(), startDrive, NodeKindSymlink)
	link.atime = now
	link.mtime = now
	link.ctime = now
	link.btime = now
	sc := contentSymlinkContent{Target: targetDentry.Node.ID().String()}
	scData, err := json.Marshal(sc)
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal symlink content")
	}
	if err := link.Write(scData, int64(len(scData))); err != nil {
		return err
	}

	if err := v.nodeOp.Create(ctx, parent.Node, link, name); err != nil {
		return err
	}
	return v.nodeOp.Symlink(ctx, link, &Dentry{
		Parent: targetDentry.Node,
		Name:   name,
		Node:   targetDentry.Node,
	})
}

// emptyDirContent returns the JSON bytes for an empty directory's
// inline content. Used by Create(Dir) so subsequent Lookup sees
// a valid directory.
func emptyDirContent() []byte {
	c := struct {
		Entries []struct{} `json:"items"`
	}{}
	b, _ := json.Marshal(c)
	return b
}

// contentSymlinkContent mirrors content.SymlinkContent. Defined
// here so this file doesn't have to import internal/content
// (the resolver/walk layer imports content; this layer avoids
// the cycle).
type contentSymlinkContent struct {
	Target string `json:"ino"`
}
