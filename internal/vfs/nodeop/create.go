package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Create implements [NodeOperation]. Inserts `child` into
// `parent`'s directory listing under `name`. The caller (vfs
// high-level command) is responsible for permission checks;
// nodeop only enforces structural invariants.
func (n *nodeOperation) Create(ctx context.Context, parent *vfs.Node, child *vfs.Node, name string) error {
	if parent.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: parent is not a directory")
	}
	if child.Drive().Compare(parent.Drive()) != 0 {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: child drive does not match parent drive")
	}

	dirContent := &content.DirContent{}
	if err := json.Unmarshal(parent.Data(), dirContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal parent directory content")
	}
	if err := dirContent.AddEntry(content.DirEntry{
		NodeID: child.ID(),
		Name:   name,
		Kind:   child.Kind(),
	}); err != nil {
		return errorx.Wrap(err, "nodeop: failed to add entry to parent directory")
	}

	newDirData, err := dirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal parent directory content")
	}
	parent.Write(newDirData, int64(len(newDirData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, parent); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update parent directory")
		}
		if err := n.repo.Write(ctx, child); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update new node")
		}
		return nil
	})
}
