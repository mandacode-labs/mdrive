package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// read — Linux vfs_read.
func (v *vfs) read(_ context.Context, dentry *fs.Dentry) ([]byte, error) {
	if dentry == nil || dentry.Node == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: read requires a dentry")
	}
	if dentry.Node.Kind() != fs.NodeKindFile {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: not a file")
	}
	return dentry.Node.Data(), nil
}
