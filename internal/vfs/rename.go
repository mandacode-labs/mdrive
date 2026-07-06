package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Rename moves an entry. Cross-drive rename is not supported.
// Linux vfs_rename.
func (v *vfs) Rename(ctx context.Context, oldParent *Dentry, oldName string, newParent *Dentry, newName string) error {
	if oldParent == nil || newParent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: rename requires parent entries")
	}
	if oldName == "" || newName == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: rename names must be non-empty")
	}
	if oldParent.Node.SuperblockID() != newParent.Node.SuperblockID() {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: cross-drive rename not supported")
	}
	if oldParent.Node.Kind() != NodeKindDirectory || newParent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: rename parents must be directories")
	}

	old, err := v.nodeOp.Lookup(ctx, oldParent.Node, oldName)
	if err != nil {
		return err
	}
	return v.nodeOp.Rename(ctx, old, newParent.Node, newName)
}
