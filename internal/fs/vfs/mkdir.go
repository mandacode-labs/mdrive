package vfs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Mkdir — Linux vfs_mkdir. Builds a fresh directory inode
// and delegates dir-entry creation + persistence to
// nodeOp.Create. Kept separate from vfs.Create since the
// inode is constructed here (no caller-provided child).
func (v *vfs) Mkdir(ctx context.Context, parent *fs.Dentry, name string) (*fs.Node, error) {
	if parent == nil || name == "" {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: mkdir requires parent and name")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: parent is not a directory")
	}
	child := fs.NewNode(uuid.New(), parent.Node.SuperblockID(), fs.NodeKindDirectory)
	if err := v.Create(ctx, parent, child, name); err != nil {
		return nil, err
	}
	return child, nil
}
