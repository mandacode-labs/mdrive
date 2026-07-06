package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Rename implements [NodeOperation].
func (n *nodeOperation) Rename(ctx context.Context, old *vfs.Dentry, newDir *vfs.Node, newName string) error {
	if old == nil || old.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil old dentry")
	}
	if newDir == nil || newDir.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: newDir is not a directory")
	}

	newDirContent := &content.DirContent{}
	if err := json.Unmarshal(newDir.Data(), newDirContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal new directory content")
	}
	if err := newDirContent.AddEntry(content.DirEntry{
		NodeID: old.Node.ID(),
		Name:   newName,
		Kind:   old.Node.Kind(),
	}); err != nil {
		return errorx.Wrap(err, "nodeop: failed to add entry to new directory")
	}
	newData, err := newDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal new directory content")
	}
	newDir.Write(newData, int64(len(newData)))

	oldDir := old.Parent
	if oldDir == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: old dentry has no parent")
	}
	oldDirContent := &content.DirContent{}
	if err := json.Unmarshal(oldDir.Data(), oldDirContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal old directory content")
	}
	if err := oldDirContent.RemoveEntry(old.Name); err != nil {
		return errorx.Wrap(err, "nodeop: failed to remove entry from old directory")
	}
	oldData, err := oldDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal old directory content")
	}
	oldDir.Write(oldData, int64(len(oldData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, oldDir); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update old parent directory")
		}
		if err := n.repo.Write(ctx, newDir); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update new parent directory")
		}
		return nil
	})
}
