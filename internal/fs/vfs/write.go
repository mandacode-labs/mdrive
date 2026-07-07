package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// write — Linux vfs_write.
func (v *vfs) Write(_ context.Context, dentry *fs.Dentry, data []byte) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: write requires a dentry")
	}
	if dentry.Node.Kind() != fs.NodeKindFile {
		return errorx.New(errorx.KindInvalidArgument, "fs: not a file")
	}
	return dentry.Node.Write(data, int64(len(data)))
}
