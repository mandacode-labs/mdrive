package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Unlink implements [NodeOperation]. Removes the directory entry
// pointing at `dentry.Node` and decrements the node's link count.
// If the link count reaches zero, the inode is destroyed — Linux's
// "last reference drops" rule.
func (n *nodeOperation) Unlink(ctx context.Context, dentry *vfs.Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil dentry")
	}
	if dentry.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: dentry has no parent")
	}
	if dentry.Parent.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: parent is not a directory")
	}
	if dentry.Node.Kind() == vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: cannot unlink a directory; use Rmdir")
	}

	if err := n.requirePerm(ctx, permission.ActionEdit, dentry.Parent.SuperblockID()); err != nil {
		return err
	}

	dirContent := &content.DirContent{}
	if err := json.Unmarshal(dentry.Parent.Data(), dirContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal parent directory content")
	}
	if err := dirContent.RemoveEntry(dentry.Name); err != nil {
		return errorx.Wrap(err, "nodeop: failed to remove entry from parent directory")
	}
	newDirData, err := dirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal parent directory content")
	}
	dentry.Parent.Write(newDirData, int64(len(newDirData)))

	dentry.Node.DecNLink()

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, dentry.Parent); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update parent directory")
		}
		if dentry.Node.NLink() == 0 {
			if err := n.repo.Destroy(ctx, dentry.Node.ID()); err != nil {
				return errorx.Wrap(err, "nodeop: failed to destroy unlinked inode")
			}
		} else {
			if err := n.repo.Write(ctx, dentry.Node); err != nil {
				return errorx.Wrap(err, "nodeop: failed to update inode link count")
			}
		}
		return nil
	})
}
