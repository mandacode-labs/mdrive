package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mkdir implements [NodeOperation].
func (n *nodeOperation) Mkdir(ctx context.Context, dentry *Dentry) error {
	// Link the new directory node to its parent directory.
	if err := dentry.Parent.AddEntry(dentry.Name, dentry.Node); err != nil {
		return err
	}

	// Check permissions for the drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, dentry.Drive.ID()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.super.Write(ctx, dentry.Node); err != nil {
			return err
		}
		if err := n.super.Write(ctx, dentry.Parent); err != nil {
			return err
		}
		return nil
	})
	return nil
}
