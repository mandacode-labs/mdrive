package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Create inserts `child` into `parent`'s directory listing
// under `name`. Caller is responsible for permission;
// nodeop enforces structural invariants only.
func (n *nodeOperation) Create(ctx context.Context, parent *fs.Node, child *fs.Node, name string) error {
	if parent.Kind() != fs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: parent is not a directory")
	}
	if child.SuperblockID() != parent.SuperblockID() {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: child superblock mismatch")
	}

	dirContent := &fs.DirContent{}
	if err := json.Unmarshal(parent.Data(), dirContent); err != nil {
		return errorx.Wrap(err, "nodeop: parent dir content")
	}
	if err := dirContent.AddEntry(fs.DirEntry{
		NodeID: child.ID(),
		Name:   name,
		Kind:   child.Kind(),
	}); err != nil {
		return errorx.Wrap(err, "nodeop: add entry")
	}

	newDirData, err := dirContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal parent dir")
	}
	parent.Write(newDirData, int64(len(newDirData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, parent); err != nil {
			return errorx.Wrap(err, "nodeop: write parent dir")
		}
		if err := n.repo.Write(ctx, child); err != nil {
			return errorx.Wrap(err, "nodeop: write child")
		}
		return nil
	})
}
