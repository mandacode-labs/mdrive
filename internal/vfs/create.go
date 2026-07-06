package vfs

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Create inserts `child` into the directory `parent` under `name`.
// Caller is responsible for resolving the path; the vfs layer
// only handles the structural insertion.
func (v *vfs) Create(ctx context.Context, parent *Dentry, child *Node, name string) error {
	if parent == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: create requires parent and name")
	}
	if child == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: child is nil")
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: parent is not a directory")
	}
	if child.SuperblockID() != parent.Node.SuperblockID() {
		return errorx.New(errorx.KindInvalidArgument, "vfs: child superblock does not match parent")
	}

	now := time.Now()
	child.atime = now
	child.mtime = now
	child.ctime = now
	child.btime = now

	return v.nodeOp.Create(ctx, parent.Node, child, name)
}
