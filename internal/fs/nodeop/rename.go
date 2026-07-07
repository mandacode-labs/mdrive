package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Rename moves a directory entry from old.Parent to newParent
// under newName. Both old.Parent and newParent must share the
// same superblock (cross-drive rename is rejected at vfs level).
func (n *nodeOperation) Rename(ctx context.Context, old *fs.Dentry, newParent *fs.Dentry, newName string) error {
	if old == nil || old.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil old dentry")
	}
	if newParent == nil || newParent.Node == nil || newParent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: newParent is not a directory")
	}
	if old.Parent == nil || old.Parent.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: old dentry has no parent")
	}

	newDirContent := &content.DirContent{}
	if err := json.Unmarshal(newParent.Node.Data(), newDirContent); err != nil {
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
	newParent.Node.Write(newData, int64(len(newData)))

	oldDirContent := &content.DirContent{}
	if err := json.Unmarshal(old.Parent.Node.Data(), oldDirContent); err != nil {
		return errorx.Wrap(err, "nodeop: old dir content")
	}
	if err := oldDirContent.RemoveEntry(old.Name); err != nil {
		return errorx.Wrap(err, "nodeop: remove old dir entry")
	}
	oldData, err := oldDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal old dir")
	}
	old.Parent.Node.Write(oldData, int64(len(oldData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, old.Parent.Node); err != nil {
			return errorx.Wrap(err, "nodeop: write old dir")
		}
		return n.repo.Write(ctx, newParent.Node)
	})
}
