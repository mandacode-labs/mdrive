package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Symlink implements [NodeOperation].
func (n *nodeOperation) Symlink(ctx context.Context, symlink *vfs.Node, target *vfs.Dentry) error {
	if symlink == nil {
		return errorx.New(errorx.KindInvalidArgument, "target node is nil")
	}
	if symlink.Kind() != vfs.NodeKindSymlink {
		return errorx.New(errorx.KindInvalidArgument, "target node is not a symlink")
	}

	// Write New Symlink Content
	c := content.NewSymlinkContent(target.Node.ID())
	data, err := c.Marshal()
	if err != nil {
		return errorx.Wrap(err, "failed to marshal symlink content")
	}
	symlink.Write(data, int64(len(data)))

	if err := n.requirePerm(ctx, permission.ActionEdit, symlink.SuperblockID()); err != nil {
		return err
	}
	if target.Parent != nil {
		if err := n.requirePerm(ctx, permission.ActionView, target.Parent.SuperblockID()); err != nil {
			return err
		}
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, symlink); err != nil {
			return errorx.Wrap(err, "failed to update symlink")
		}
		return nil
	})
	return nil
}
