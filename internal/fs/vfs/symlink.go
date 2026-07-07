package vfs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// symlink — Linux vfs_symlink.
func (v *vfs) Symlink(ctx context.Context, linkParent *fs.Dentry, linkName string, targetID uuid.UUID) (*fs.Node, error) {
	if linkParent == nil || linkName == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: symlink requires parent and name")
	}
	if linkParent.Node.Kind() != fs.NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: link parent is not a directory")
	}
	sc := &content.SymlinkContent{NodeID: targetID}
	data, err := sc.Marshal()
	if err != nil {
		return nil, errorx.Wrap(err, "fs: symlink content", errorx.KindInternal)
	}
	link := fs.NewNode(uuid.New(), linkParent.Node.SuperblockID(), fs.NodeKindSymlink)
	if err := link.Write(data, int64(len(data))); err != nil {
		return nil, err
	}
	if err := v.nodeOp.Create(ctx, linkParent.Node, link, linkName); err != nil {
		return nil, err
	}
	if err := v.nodeOp.Symlink(ctx, link, &fs.Dentry{
		DriveID: linkParent.DriveID,
		Parent:  linkParent,
		Name:    linkName,
		Node:    link,
	}); err != nil {
		return nil, err
	}
	return link, nil
}
