package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Rmdir removes an empty directory. POSIX: refuse if
// non-empty.
func (n *nodeOperation) Rmdir(ctx context.Context, dentry *fs.Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil dentry")
	}
	if dentry.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: dentry has no parent")
	}
	if dentry.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: target is not a directory")
	}

	dirContent := &fs.DirContent{}
	if err := json.Unmarshal(dentry.Node.Data(), dirContent); err != nil {
		return errorx.Wrap(err, "nodeop: target dir content")
	}
	if len(dirContent.Entries) > 0 {
		return errorx.New(errorx.KindFailedPrecondition, "nodeop: directory not empty")
	}

	parentContent := &fs.DirContent{}
	if err := json.Unmarshal(dentry.Parent.Node.Data(), parentContent); err != nil {
		return errorx.Wrap(err, "nodeop: parent dir content")
	}
	if err := parentContent.RemoveEntry(dentry.Name); err != nil {
		return errorx.Wrap(err, "nodeop: remove parent entry")
	}
	data, err := parentContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal parent dir")
	}
	dentry.Parent.Node.Write(data, int64(len(data)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, dentry.Parent.Node); err != nil {
			return errorx.Wrap(err, "nodeop: write parent dir")
		}
		return n.repo.Destroy(ctx, dentry.Node.ID())
	})
}
