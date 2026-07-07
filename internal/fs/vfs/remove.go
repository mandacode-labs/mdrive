package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// removeRecursive empties a directory tree.
func (v *vfs) removeRecursive(ctx context.Context, dentry *fs.Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "fs: recursive target is nil")
	}
	if dentry.Node.Kind() == fs.NodeKindMount {
		src, err := v.followMount(ctx, dentry.Node)
		if err != nil {
			return err
		}
		return v.removeRecursive(ctx, src)
	}
	if dentry.Node.Kind() != fs.NodeKindDirectory {
		return nil
	}
	var dc content.DirContent
	if err := json.Unmarshal(dentry.Node.Data(), &dc); err != nil {
		return errorx.Wrap(err, "fs: dir content", errorx.KindInternal)
	}
	for _, e := range dc.Entries {
		childDentry, err := v.nodeOp.Lookup(ctx, dentry.Node, e.Name)
		if err != nil {
			return err
		}
		if err := v.removeRecursive(ctx, childDentry); err != nil {
			return err
		}
	}
	return nil
}
