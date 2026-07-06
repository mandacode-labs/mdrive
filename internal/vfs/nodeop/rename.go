package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Rename implements [NodeOperation].
func (n *nodeOperation) Rename(ctx context.Context, old *vfs.Dentry, newDir *vfs.Node, newName string) error {
	// Read the content of the new parent directory and unmarshal it into a DirContent struct.
	newDirData := newDir.Data()
	newDirContent := &content.DirContent{}
	if err := json.Unmarshal(newDirData, newDirContent); err != nil {
		return errorx.Wrap(err, "failed to unmarshal new directory content")
	}

	// Add the new entry to the new parent directory's content.
	err := newDirContent.AddEntry(content.DirEntry{
		NodeID: old.Node.ID(),
		Name:   newName,
		Kind:   old.Node.Kind(),
	})
	if err != nil {
		return errorx.Wrap(err, "failed to add entry to new directory content")
	}

	// Marshal the updated new directory content and write it back to the new parent directory node.
	newNewDirData, err := newDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "failed to marshal new directory content")
	}
	newDir.Write(newNewDirData, int64(len(newNewDirData)))

	// Remove the old entry from the old parent directory's content.
	oldDir := old.Parent
	oldDirData := oldDir.Data()
	oldDirContent := &content.DirContent{}
	if err := json.Unmarshal(oldDirData, oldDirContent); err != nil {
		return errorx.Wrap(err, "failed to unmarshal old directory content")
	}
	if err := oldDirContent.RemoveEntry(old.Name); err != nil {
		return errorx.Wrap(err, "failed to remove entry from old directory content")
	}
	// Marshal the updated old directory content and write it back to the old parent directory node.
	newOldDirData, err := oldDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "failed to marshal old directory content")
	}
	oldDir.Write(newOldDirData, int64(len(newOldDirData)))

	// Check permissions for the drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, oldDir.Drive()); err != nil {
		return err
	}
	if err := n.requirePerm(ctx, permission.ActionEdit, newDir.Drive()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, oldDir); err != nil {
			return errorx.Wrap(err, "failed to update old parent directory")
		}
		if err := n.repo.Write(ctx, newDir); err != nil {
			return errorx.Wrap(err, "failed to update new parent directory")
		}
		return nil
	})

	return nil
}
