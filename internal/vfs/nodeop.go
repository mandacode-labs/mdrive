package vfs

import "context"

// NodeOperation is the inode-level primitive set. Mirrors Linux
// inode_operations: each callback mutates the inode tree in a
// single step. High-level fs commands (Mkdir, Ls, Cat, ...) live
// in the vfs layer, not here.
//
// Each callback takes ctx explicitly so vfs can thread it from
// the system call layer. The data-access layer uses
// entx.FromContext to pick the right client (tx or non-tx).
type NodeOperation interface {
	Lookup(ctx context.Context, dir *Node, name string) (*Dentry, error)
	Create(ctx context.Context, parent *Node, child *Node, name string) error
	Link(ctx context.Context, dentry *Dentry, linkDir *Node, linkName string) error
	Symlink(ctx context.Context, symlink *Node, target *Dentry) error
	Unlink(ctx context.Context, dentry *Dentry) error
	Rmdir(ctx context.Context, dentry *Dentry) error
	Rename(ctx context.Context, old *Dentry, newDir *Node, newName string) error
}
