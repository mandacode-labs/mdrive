package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

type NodeOperation interface {
	Mknod(ctx context.Context, dir *node.Node, ino *node.Node, name string) error
	Link(ctx context.Context, dentry *Dentry, linkDir *node.Node, linkName string) error
	Symlink(ctx context.Context, symlink *node.Node, target *Dentry) error
	Unlink(ctx context.Context, dentry *Dentry) error
	Rmdir(ctx context.Context, dentry *Dentry) error
	Rename(ctx context.Context, old *Dentry, newDir *node.Node, newName string) error
	Lookup(ctx context.Context, dir *node.Node, name string) (*Dentry, error)
}
