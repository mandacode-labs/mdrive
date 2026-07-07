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
