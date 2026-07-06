package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Read returns the inline data of a file-kind node.
// Linux vfs_read.
func (v *vfs) Read(ctx context.Context, dentry *Dentry) ([]byte, error) {
	if dentry == nil || dentry.Node == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: read requires a dentry")
	}
	if dentry.Node.Kind() != NodeKindFile {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a file")
	}
	return dentry.Node.Data(), nil
}
