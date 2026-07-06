package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Link implements [NodeOperation].
func (n *nodeOperation) Link(ctx context.Context, dentry *vfs.Dentry, linkDir *vfs.Node, linkName string) error {
	if linkDir.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: linkDir is not a directory")
	}

	linkDirContent := &content.DirContent{}
	if err := json.Unmarshal(linkDir.Data(), linkDirContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal link directory content")
	}
	if err := linkDirContent.AddEntry(content.DirEntry{
		NodeID: dentry.Node.ID(),
		Name:   linkName,
		Kind:   dentry.Node.Kind(),
	}); err != nil {
		return errorx.Wrap(err, "nodeop: failed to add entry to link directory")
	}

	dentry.Node.IncNLink()

	newData, err := linkDirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal link directory content")
	}
	linkDir.Write(newData, int64(len(newData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, linkDir); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update link directory")
		}
		return nil
	})
}
