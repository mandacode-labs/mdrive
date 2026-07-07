// Package vfs is the inode layer of the fs subsystem. It
// mirrors Linux's vfs_* functions. fs.Service handles path
// lookup and permission checks; vfs operates on already-
// resolved *Dentry.
package vfs

import (
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// vfs is the concrete implementation of fs.VFS.
type vfs struct {
	nodeOp  fs.NodeOperation
	superop fs.SuperOperation
}

// New constructs an fs.VFS implementation.
func New(nodeOp fs.NodeOperation, superop fs.SuperOperation) fs.VFS {
	return &vfs{nodeOp: nodeOp, superop: superop}
}

var _ fs.VFS = (*vfs)(nil)
