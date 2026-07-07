package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Create — Linux vfs_create. Inserts a pre-constructed child
// inode into `parent` under `name`. Used by Service when the
// caller has already built the inode (CreateFile /
// CreateObject / Symlink).
func (v *vfs) Create(ctx context.Context, parent *fs.Dentry, child *fs.Node, name string) error {
	if parent == nil || name == "" || child == nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: create requires parent, child, name")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "fs: parent is not a directory")
	}
	if child.SuperblockID() != parent.Node.SuperblockID() {
		return errorx.New(errorx.KindInvalidArgument, "fs: child superblock mismatch")
	}
	return v.nodeOp.Create(ctx, parent.Node, child, name)
}
