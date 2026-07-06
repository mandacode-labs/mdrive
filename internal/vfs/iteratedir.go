package vfs

import (
	"context"
	"encoding/json"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// IterateDir returns the entries of a directory. Linux
// iterate_dir / getdents64.
//
// Only direct children are returned. The parent dir's
// DirContent is read inline (Node.Data()), so no further DB
// lookups are needed.
func (v *vfs) IterateDir(ctx context.Context, driveID string, path string) ([]DirEntry, error) {
	startDrive, err := ulid.Parse(driveID)
	if err != nil {
		return nil, errorx.Wrap(err, "vfs: invalid drive id", errorx.KindInvalidArgument)
	}
	dentry, err := v.walk(ctx, startDrive, path, true)
	if err != nil {
		return nil, err
	}
	if dentry.Node.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a directory")
	}
	dc := &content.DirContent{}
	if err := json.Unmarshal(dentry.Node.Data(), dc); err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to decode directory")
	}
	out := make([]DirEntry, 0, len(dc.Entries))
	for _, e := range dc.Entries {
		out = append(out, DirEntry{
			InodeID: e.NodeID,
			Name:    e.Name,
			Kind:    NodeKind(e.Kind),
		})
	}
	return out, nil
}
