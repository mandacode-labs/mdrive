package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/vfs/nodeop/content"
)

// Symlink implements [NodeOperation].
func (n *nodeOperation) Symlink(ctx context.Context, symlink *node.Node, target *Dentry) error {
	if symlink == nil {
		return errorx.New(errorx.KindInvalidArgument, "target node is nil")
	}
	if symlink.Kind() != node.NodeKindSymlink {
		return errorx.New(errorx.KindInvalidArgument, "target node is not a symlink")
	}

	// Write New Symlink Content
	c := content.NewSymlinkContent(target.Node.ID())
	data, err := c.Marshal()
	if err != nil {
		return errorx.Wrap(err, "failed to marshal symlink content")
	}
	symlink.Write(data, int64(len(data)))

	// Check permissions for the symlink's drive and the target node's drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, symlink.DriveID()); err != nil {
		return err
	}
	if err := n.requirePerm(ctx, permission.ActionView, target.Parent.DriveID()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.super.Write(ctx, symlink); err != nil {
			return errorx.Wrap(err, "failed to update symlink")
		}
		return nil
	})
	return nil
}
