package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Rename moves a directory entry from old.Parent to newDir
// under newName.
func (n *nodeOperation) Rename(ctx context.Context, old *fs.Dentry, newDir *fs.Node, newName string) error {
	if old == nil || old.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil old dentry")
	}
	if newDir == nil || newDir.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: newDir is not a directory")
	}
	oldDir := old.Parent
	if oldDir == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: old dentry has no parent")
	}

	newDirContent := &content.DirContent{}
	if err := json.Unmarshal(newDir.Data(), newDirContent); err != nil {
		return errorx.Wrap(err, "nodeop: new dir content")
	}
	if err := newDirContent.AddEntry(content.DirEntry{
		NodeID: old.Node.ID(),
		Name:   newName,
		Kind:   old.Node.Kind(),
	}); err != nil {
		return errorx.Wrap(err, "nodeop: add new dir entry")
	}
	newData, err := newDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal new dir")
	}
	newDir.Write(newData, int64(len(newData)))

	oldDirContent := &content.DirContent{}
	if err := json.Unmarshal(oldDir.Data(), oldDirContent); err != nil {
		return errorx.Wrap(err, "nodeop: old dir content")
	}
	if err := oldDirContent.RemoveEntry(old.Name); err != nil {
		return errorx.Wrap(err, "nodeop: remove old dir entry")
	}
	oldData, err := oldDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal old dir")
	}
	oldDir.Write(oldData, int64(len(oldData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, oldDir); err != nil {
			return errorx.Wrap(err, "nodeop: write old dir")
		}
		return n.repo.Write(ctx, newDir)
	})
}
