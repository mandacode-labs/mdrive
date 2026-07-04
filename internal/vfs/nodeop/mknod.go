package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mknod implements [NodeOperation].
func (n *nodeOperation) Mknod(ctx context.Context, dentry *Dentry) error {
	// Link the node to its parent directory.
	dir := dentry.Parent
	err := dir.AddEntry(dentry.Name, dentry.Node)
	if err != nil {
		return errorx.Wrap(err, "failed to link node to parent")
	}

	// Check permissions for the drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, dentry.Drive.ID()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		// Create the node in the database.
		if err := n.super.Write(ctx, dentry.Node); err != nil {
			return errorx.Wrap(err, "failed to create node")
		}
		if err := n.super.Write(ctx, dir); err != nil {
			return errorx.Wrap(err, "failed to update parent directory")
		}
		return nil
	})
	return nil
}
