package vfs

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Symlink creates a link at `linkName` under `linkParent`
// pointing at `targetID`. Target id is stored inline as
// content.SymlinkContent.
func (v *vfs) Symlink(ctx context.Context, linkParent *Dentry, linkName string, targetID uuid.UUID) error {
	if linkParent == nil || linkName == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: symlink requires parent and name")
	}
	if linkParent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: link parent is not a directory")
	}

	sc := &content.SymlinkContent{NodeID: targetID}
	data, err := sc.Marshal()
	if err != nil {
		return errorx.Wrap(err, "vfs: failed to marshal symlink content", errorx.KindInternal)
	}

	now := time.Now()
	link := NewNode(uuid.New(), linkParent.Node.SuperblockID(), NodeKindSymlink)
	link.atime = now
	link.mtime = now
	link.ctime = now
	link.btime = now
	if err := link.Write(data, int64(len(data))); err != nil {
		return err
	}

	if err := v.nodeOp.Create(ctx, linkParent.Node, link, linkName); err != nil {
		return err
	}
	return v.nodeOp.Symlink(ctx, link, &Dentry{
		Parent: linkParent.Node,
		Name:   linkName,
		Node:   link,
	})
}
