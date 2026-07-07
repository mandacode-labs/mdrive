package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// iterate — Linux iterate_dir.
func (v *vfs) Iterate(_ context.Context, parent *fs.Dentry) ([]fs.DirEntry, error) {
	if parent == nil || parent.Node == nil {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: iteratedir requires a parent")
	}
	if parent.Node.Kind() != fs.NodeKindDirectory {
		return nil, errorx.New(errorx.KindInvalidArgument, "fs: not a directory")
	}
	var dc fs.DirContent
	if err := json.Unmarshal(parent.Node.Data(), &dc); err != nil {
		return nil, errorx.Wrap(err, "fs: dir content", errorx.KindInternal)
	}
	return dc.Entries, nil
}
