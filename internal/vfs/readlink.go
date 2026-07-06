package vfs

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Readlink returns the symlink target's inode id. Linux
// readlink(2), adapted for mdrive's graph-based content.
func (v *vfs) Readlink(ctx context.Context, dentry *Dentry) (uuid.UUID, error) {
	if dentry == nil || dentry.Node == nil {
		return uuid.Nil, errorx.New(errorx.KindInvalidArgument, "vfs: readlink requires a dentry")
	}
	if dentry.Node.Kind() != NodeKindSymlink {
		return uuid.Nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a symlink")
	}
	var sc content.SymlinkContent
	if err := json.Unmarshal(dentry.Node.Data(), &sc); err != nil {
		return uuid.Nil, errorx.Wrap(err, "vfs: invalid symlink content", errorx.KindInternal)
	}
	return sc.NodeID, nil
}
