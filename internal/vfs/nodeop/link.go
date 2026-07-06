package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Link implements [NodeOperation].
func (n *nodeOperation) Link(ctx context.Context, dentry *vfs.Dentry, linkDir *vfs.Node, linkName string) error {
	// Check if linkDir is a directory
	if linkDir.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "linkDir is not a directory")
	}

	// Read the content of the link directory and unmarshal it into a DirContent struct
	linkDirData := linkDir.Data()
	linkDirContent := &content.DirContent{}
	if err := json.Unmarshal(linkDirData, linkDirContent); err != nil {
		return errorx.Wrap(err, "failed to unmarshal link directory content")
	}

	// Add the new entry to the link directory's content
	err := linkDirContent.AddEntry(content.DirEntry{
		NodeID: dentry.Node.ID(),
		Name:   linkName,
		Kind:   dentry.Node.Kind(),
	})
	if err != nil {
		return errorx.Wrap(err, "failed to add entry to link directory content")
	}

	// Increment the link count of the target node
	dentry.Node.IncNLink()

	// Marshal the updated link directory content and write it back to the link directory node
	newLinkDirData, err := linkDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "failed to marshal link directory content")
	}
	linkDir.Write(newLinkDirData, int64(len(newLinkDirData)))

	// Check permissions for the drive
	if err := n.requirePerm(ctx, permission.ActionEdit, linkDir.Drive()); err != nil {
		return err
	}
	if err := n.requirePerm(ctx, permission.ActionEdit, dentry.Parent.Drive()); err != nil {
		return err
	}

	// Save the changes to the database
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, linkDir); err != nil {
			return errorx.Wrap(err, "failed to update link directory")
		}
		return nil
	})
	return nil
}
