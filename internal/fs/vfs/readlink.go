package vfs

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// readlink — Linux vfs_readlink.
func (v *vfs) Readlink(_ context.Context, dentry *fs.Dentry) (uuid.UUID, error) {
	if dentry == nil || dentry.Node == nil {
		return uuid.Nil, errorx.New(errorx.KindInvalidArgument, "fs: readlink requires a dentry")
	}
	if dentry.Node.Kind() != fs.NodeKindSymlink {
		return uuid.Nil, errorx.New(errorx.KindInvalidArgument, "fs: not a symlink")
	}
	var sc fs.SymlinkContent
	if err := json.Unmarshal(dentry.Node.Data(), &sc); err != nil {
		return uuid.Nil, errorx.Wrap(err, "fs: symlink content", errorx.KindInternal)
	}
	return sc.NodeID, nil
}
