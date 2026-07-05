package vfs

import (
	"context"
)

type NodeOperation interface {
	Mknod(ctx context.Context, dir *Node, ino *Node, name string) error
	Link(ctx context.Context, dentry *Dentry, linkDir *Node, linkName string) error
	Symlink(ctx context.Context, symlink *Node, target *Dentry) error
	Unlink(ctx context.Context, dentry *Dentry) error
	Rmdir(ctx context.Context, dentry *Dentry) error
	Rename(ctx context.Context, old *Dentry, newDir *Node, newName string) error
	Lookup(ctx context.Context, dir *Node, name string) (*Dentry, error)
}
