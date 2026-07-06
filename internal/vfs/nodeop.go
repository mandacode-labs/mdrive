package vfs

import (
	"context"

	"github.com/google/uuid"
)

// NodeOperation is the inode-level callback set. Mirrors Linux
// inode_operations: lookup + the structural mutations. Permission
// is the caller's responsibility.
type NodeOperation interface {
	Get(ctx context.Context, id uuid.UUID) (*Node, error)
	Lookup(ctx context.Context, dir *Node, name string) (*Dentry, error)
	Create(ctx context.Context, parent *Node, child *Node, name string) error
	Link(ctx context.Context, dentry *Dentry, linkDir *Node, linkName string) error
	Symlink(ctx context.Context, symlink *Node, target *Dentry) error
	Unlink(ctx context.Context, dentry *Dentry) error
	Rmdir(ctx context.Context, dentry *Dentry) error
	Rename(ctx context.Context, old *Dentry, newDir *Node, newName string) error
}
