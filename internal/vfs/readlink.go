package vfs

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Readlink returns the symlink target's inode id. Linux readlink(2),
// adapted: graph-based (target's uuid.UUID) rather than raw path.
func (v *vfs) Readlink(ctx context.Context, driveID string, path string) (uuid.UUID, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return uuid.Nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, err := v.walk(ctx, startDrive, path, false)
	if err != nil {
		return uuid.Nil, err
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
