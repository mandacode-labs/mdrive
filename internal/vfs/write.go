package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Write sets the inline data of a file-kind node.
// Linux vfs_write.
func (v *vfs) Write(ctx context.Context, dentry *Dentry, data []byte) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: write requires a dentry")
	}
	if dentry.Node.Kind() != NodeKindFile {
		return errorx.New(errorx.KindInvalidArgument, "vfs: not a file")
	}
	return dentry.Node.Write(data, int64(len(data)))
}
