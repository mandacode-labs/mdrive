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

// Create creates an empty inode. Linux vfs_create + vfs_mkdir +
// vfs_mknod, unified via kind. No kind-specific data is set
// here; that is the job of the kind-specific command
// (Write / WriteObject / Mount / Symlink).
//
// A directory-kind inode is initialized with an empty
// DirContent so subsequent Lookup/Mknod on it succeeds.
func (v *vfs) Create(ctx context.Context, driveID string, path string, kind NodeKind) (*Node, error) {
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

	if kind == NodeKindDirectory {
		if err := child.Write(emptyDirContent(), int64(len(emptyDirContent()))); err != nil {
			return nil, err
		}
	}

	if err := v.nodeOp.Create(ctx, parent.Node, child, name); err != nil {
		return nil, err
	}
	return child, nil
}

// Symlink creates a symlink at linkPath pointing at target.
// vfs stores the target id inline via content.SymlinkContent.
func (v *vfs) Symlink(ctx context.Context, driveID string, target string, linkPath string) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	targetDentry, err := v.resolveTarget(ctx, driveID, target, permission.ActionView)
	if err != nil {
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
// inline content.
func emptyDirContent() []byte {
	c := struct {
		Items []struct{} `json:"items"`
	}{}
	b, _ := json.Marshal(c)
	return b
}

// contentSymlinkContent mirrors content.SymlinkContent. Kept
// local so this file doesn't depend on internal/content's import
// chain.
type contentSymlinkContent struct {
	Target string `json:"ino"`
}
