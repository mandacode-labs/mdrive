package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Symlink creates a link at linkPath pointing at target. Target
// id is stored inline via content.SymlinkContent.
func (v *vfs) Symlink(ctx context.Context, driveID string, target string, linkPath string) error {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	targetDentry, _, err := v.walkEntry(ctx, startDrive, target, true)
	if err != nil {
		return err
	}
	linkDentry, sb, err := v.walkEntry(ctx, startDrive, linkPath, false)
	if err != nil {
		return err
	}
	if linkDentry.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link path has no parent")
	}
	if linkDentry.Parent.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link parent is not a directory")
	}
	if err := v.checkPerm(ctx, permission.ActionEdit, startDrive); err != nil {
		return err
	}

	now := time.Now()
	link := NewNode(uuid.New(), sb.ID(), NodeKindSymlink)
	link.atime = now
	link.mtime = now
	link.ctime = now
	link.btime = now
	sc := &content.SymlinkContent{NodeID: targetDentry.Node.ID()}
	scData, err := sc.Marshal()
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal symlink content")
	}
	if err := link.Write(scData, int64(len(scData))); err != nil {
		return err
	}

	if err := v.nodeOp.Create(ctx, linkDentry.Parent, link, linkDentry.Name); err != nil {
		return err
	}
	return v.nodeOp.Symlink(ctx, link, &Dentry{
		Parent: targetDentry.Node,
		Name:   target,
		Node:   targetDentry.Node,
	})
}
