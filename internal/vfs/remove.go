package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Remove deletes `name` under `parent`. Recursive: cascade
// children first. Mount nodes cross into the source drive.
// Drive lifecycle is the caller's concern.
func (v *vfs) Remove(ctx context.Context, parent *Dentry, name string, opts RemoveOpts) error {
	if parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: parent is nil")
	}
	if name == "" {
		return errorx.New(errorx.KindInvalidArgument, "vfs: name is empty")
	}

	dentry, err := v.nodeOp.Lookup(ctx, parent.Node, name)
	if err != nil {
		return err
	}

	if !opts.Recursive {
		switch dentry.Node.Kind() {
		case NodeKindDirectory:
			return v.Rmdir(ctx, parent, name)
		default:
			return v.Unlink(ctx, parent, name)
		}
	}

	if err := v.removeRecursive(ctx, dentry); err != nil {
		return err
	}
	switch dentry.Node.Kind() {
	case NodeKindDirectory:
		return v.Rmdir(ctx, parent, name)
	default:
		return v.Unlink(ctx, parent, name)
	}
}

// removeRecursive empties a directory tree. Mounts are
// followed into the source drive.
func (v *vfs) removeRecursive(ctx context.Context, dentry *Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "vfs: recursive target is nil")
	}

	if dentry.Node.Kind() == NodeKindMount {
		src, err := v.walkMount(ctx, dentry.Node)
		if err != nil {
			return err
		}
		return v.removeRecursive(ctx, src)
	}

	if dentry.Node.Kind() != NodeKindDirectory {
		return nil
	}

	var dc content.DirContent
	if err := json.Unmarshal(dentry.Node.Data(), &dc); err != nil {
		return errorx.Wrap(err, "vfs: failed to unmarshal directory content", errorx.KindInternal)
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
