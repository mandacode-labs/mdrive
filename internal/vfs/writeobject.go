package vfs

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// WriteObject is a structural variant: the caller has built
// the child node with content.ObjectContent already set
// (e.g. after a successful S3 upload).
func (v *vfs) WriteObject(ctx context.Context, parent *Dentry, child *Node, name string) error {
	if parent == nil || child == nil || name == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: writeobject requires parent, child, name")
	}
	if child.Kind() != NodeKindObject {
		return errorx.New(errorx.KindInvalidArgument, "vfs: child is not Object kind")
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "vfs: parent is not a directory")
	}

	now := time.Now()
	child.atime = now
	child.mtime = now
	child.ctime = now
	child.btime = now

	return v.nodeOp.Create(ctx, parent.Node, child, name)
}
