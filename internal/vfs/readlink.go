package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Readlink returns the symlink target's inode id as a string.
// Linux vfs_readlink.
func (v *vfs) Readlink(ctx context.Context, driveID string, path string) (string, error) {
	target, err := v.resolveTarget(ctx, driveID, path, permission.ActionView)
	if err != nil {
		return "", err
	}
	if target.Parent == nil {
		return "", errorx.New(errorx.KindInvalidArgument, "vfs: path has no parent")
	}
	dentry, err := v.nodeOp.Lookup(ctx, target.Parent, target.Name)
	if err != nil {
		return "", err
	}
	if dentry.Node.Kind() != NodeKindSymlink {
		return "", errorx.New(errorx.KindInvalidArgument, "vfs: not a symlink")
	}
	var sc content.SymlinkContent
	if err := json.Unmarshal(dentry.Node.Data(), &sc); err != nil {
		return "", errorx.Wrap(err, "vfs: invalid symlink content")
	}
	return sc.NodeID.String(), nil
}