package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Mknod implements [NodeOperation].
func (n *nodeOperation) Mknod(ctx context.Context, dir *vfs.Node, ino *vfs.Node, name string) error {
	// Check if the parent directory is a directory.
	if dir.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "parent node is not a directory")
	}

	// Check if the node drive ID matches the parent directory drive ID.
	if ino.Drive().Compare(dir.Drive()) != 0 {
		return errorx.New(errorx.KindInvalidArgument, "node drive ID does not match parent directory drive ID")
	}

	// Check if the name already exists in the parent directory.
	dirData := dir.Data()
	dirContent := &content.DirContent{}
	if err := json.Unmarshal(dirData, dirContent); err != nil {
		return errorx.Wrap(err, "failed to unmarshal directory content")
	}

	// Add the new entry to the parent directory's content.
	err := dirContent.AddEntry(content.DirEntry{
		NodeID: ino.ID(),
		Name:   name,
		Kind:   ino.Kind(),
	})
	if err != nil {
		return errorx.Wrap(err, "failed to add entry to directory content")
	}

	// Marshal the updated directory content and write it back to the parent directory node.
	newDirData, err := dirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "failed to marshal directory content")
	}
	dir.Write(newDirData, int64(len(newDirData)))

	// Check permissions for the drive.
	if err := n.requirePerm(ctx, permission.ActionEdit, dir.Drive()); err != nil {
		return err
	}

	// Save the changes to the database.
	n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, dir); err != nil {
			return errorx.Wrap(err, "failed to update parent directory")
		}
		if err := n.repo.Write(ctx, ino); err != nil {
			return errorx.Wrap(err, "failed to update new node")
		}
		return nil
	})
	return nil
}
