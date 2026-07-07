package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// rename — Linux vfs_rename.
func (v *vfs) rename(ctx context.Context, oldParent *fs.Dentry, oldName string, newParent *fs.Dentry, newName string) error {
	if oldParent == nil || newParent == nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: rename requires parent entries")
	}
	if oldName == "" || newName == "" {
		return errorx.New(errorx.KindInvalidArgument, "fs: rename names must be non-empty")
	}
	if oldParent.Node.SuperblockID() != newParent.Node.SuperblockID() {
		return errorx.New(errorx.KindFailedPrecondition, "fs: cross-drive rename not supported")
	}
	if oldParent.Node.Kind() != fs.NodeKindDirectory || newParent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: rename parents must be directories")
	}
	old, err := v.nodeOp.Lookup(ctx, oldParent.Node, oldName)
	if err != nil {
		return err
	}
	return v.nodeOp.Rename(ctx, old, newParent.Node, newName)
}
