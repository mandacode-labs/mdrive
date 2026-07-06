package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// IterateDir returns the direct children of a directory-kind
// node. Linux iterate_dir / getdents64.
func (v *vfs) IterateDir(ctx context.Context, parent *Dentry) ([]DirEntry, error) {
	if parent == nil || parent.Node == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: iteratedir requires a parent")
	}
	if parent.Node.Kind() != NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a directory")
	}
	var dc content.DirContent
	if err := json.Unmarshal(parent.Node.Data(), &dc); err != nil {
		return nil, errorx.Wrap(err, "vfs: failed to decode directory", errorx.KindInternal)
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
