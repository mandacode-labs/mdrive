package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Link implements [NodeOperation].
func (n *nodeOperation) Link(ctx context.Context, targetSymlink *Dentry, targetNode *Dentry) error {
	symlink := targetSymlink.Node
	if symlink == nil {
		return errorx.New(errorx.KindInvalidArgument, "target node is nil")
	}
	if symlink.Kind() != node.NodeKindSymlink {
		return errorx.New(errorx.KindInvalidArgument, "target node is not a symlink")
	}
	// Link the target node to the symlink's parent directory.
	if err := symlink.WriteSymlink(targetNode.Node.ID()); err != nil {
		return errorx.Wrap(err, "failed to write symlink target")
	}

	// Check permissions for the symlink's drive and the target node's drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, targetSymlink.Drive.ID()); err != nil {
		return err
	}
	if err := n.requirePerm(ctx, permission.ActionEdit, targetNode.Drive.ID()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.super.Write(ctx, symlink); err != nil {
			return errorx.Wrap(err, "failed to update symlink")
		}
		if err := n.super.Write(ctx, targetSymlink.Parent); err != nil {
			return errorx.Wrap(err, "failed to update symlink's parent directory")
		}
		return nil
	})
	return nil
}
