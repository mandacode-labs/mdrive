package vfs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// create — Linux vfs_create.
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

// mkdir — Linux vfs_mkdir.
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
