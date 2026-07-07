package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Link adds a directory entry pointing at the same inode as
// `dentry`. Increments nlink and persists both the new
// directory entry and the bumped inode in a single tx.
func (n *nodeOperation) Link(ctx context.Context, dentry *fs.Dentry, linkDir *fs.Dentry, linkName string) error {
	if linkDir.Node.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: linkDir is not a directory")
	}

	dir := &content.DirContent{}
	if err := json.Unmarshal(linkDir.Node.Data(), dir); err != nil {
		return errorx.Wrap(err, "nodeop: link dir content")
	}
	if err := dir.AddEntry(content.DirEntry{
		NodeID: dentry.Node.ID(),
		Name:   linkName,
		Kind:   dentry.Node.Kind(),
	}); err != nil {
		return errorx.Wrap(err, "nodeop: add entry")
	}

	dentry.Node.IncNLink()

	data, err := dir.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal link dir")
	}
	linkDir.Node.Write(data, int64(len(data)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, linkDir.Node); err != nil {
			return errorx.Wrap(err, "nodeop: write link dir")
		}
		return n.repo.Write(ctx, dentry.Node)
	})
}
