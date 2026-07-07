package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// iterate — Linux iterate_dir.
func (v *vfs) iterate(_ context.Context, parent *fs.Dentry) ([]fs.DirEntry, error) {
	if parent == nil || parent.Node == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: iteratedir requires a parent")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: not a directory")
	}
	var dc content.DirContent
	if err := json.Unmarshal(parent.Node.Data(), &dc); err != nil {
		return nil, errorx.Wrap(err, "fs: dir content", errorx.KindInternal)
	}
	out := make([]fs.DirEntry, 0, len(dc.Entries))
	for _, e := range dc.Entries {
		out = append(out, fs.DirEntry{
			InodeID: e.NodeID,
			Name:    e.Name,
			Kind:    fs.NodeKind(e.Kind),
		})
	}
	return out, nil
}
