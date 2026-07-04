package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Rename implements [NodeOperation].
func (n *nodeOperation) Rename(ctx context.Context, oldDentry *Dentry, newDentry *Dentry) error {
	// Check if the old and new nodes are the same.
	if oldDentry.Node.ID() != newDentry.Node.ID() {
		return errorx.New(errorx.KindInvalidArgument, "old and new nodes are not the same")
	}

	// Add node to new parent directory
	if err := newDentry.Parent.AddEntry(newDentry.Name, oldDentry.Node); err != nil {
		return err
	}
	// Remove node from old parent directory
	if err := oldDentry.Parent.RemoveEntry(oldDentry.Name); err != nil {
		return err
	}

	// Check permissions for the drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, oldDentry.Drive.ID()); err != nil {
		return err
	}
	if err := n.requirePerm(ctx, permission.ActionEdit, newDentry.Drive.ID()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.super.Write(ctx, oldDentry.Parent); err != nil {
			return err
		}
		if err := n.super.Write(ctx, newDentry.Parent); err != nil {
			return err
		}
		return nil
	})
	return nil
}
