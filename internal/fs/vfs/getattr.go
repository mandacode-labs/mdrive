package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// getattr — Linux inode_getattr.
func (v *vfs) Getattr(_ context.Context, dentry *fs.Dentry) (fs.Stat, error) {
	if dentry == nil {
		return fs.Stat{}, errorx.New(errorx.KindInvalidArgument, "fs: getattr requires a dentry")
	}
	return fs.NodeToStat(dentry.Node), nil
}
