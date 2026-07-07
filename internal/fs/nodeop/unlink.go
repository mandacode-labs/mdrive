package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Unlink removes the directory entry pointing at
// `dentry.Node`, decrements its link count, and destroys the
// inode if the count reaches zero.
func (n *nodeOperation) Unlink(ctx context.Context, dentry *fs.Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil dentry")
	}
	if dentry.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: dentry has no parent")
	}
	if dentry.Parent.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: parent is not a directory")
	}
	if dentry.Node.Kind() == fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: cannot unlink a directory")
	}

	dirContent := &fs.DirContent{}
	if err := json.Unmarshal(dentry.Parent.Node.Data(), dirContent); err != nil {
		return errorx.Wrap(err, "nodeop: parent dir content")
	}
	if err := dirContent.RemoveEntry(dentry.Name); err != nil {
		return errorx.Wrap(err, "nodeop: remove parent entry")
	}
	data, err := dirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal parent dir")
	}
	dentry.Parent.Node.Write(data, int64(len(data)))

	dentry.Node.DecNLink()

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, dentry.Parent.Node); err != nil {
			return errorx.Wrap(err, "nodeop: write parent dir")
		}
		if dentry.Node.NLink() == 0 {
			return n.repo.Destroy(ctx, dentry.Node.ID())
		}
		return n.repo.Write(ctx, dentry.Node)
	})
}
