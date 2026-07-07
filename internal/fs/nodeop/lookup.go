package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Lookup resolves `name` under `dir` to a Dentry.
func (n *nodeOperation) Lookup(ctx context.Context, dir *fs.Node, name string) (*fs.Dentry, error) {
	dirContent := &content.DirContent{}
	if err := json.Unmarshal(dir.Data(), dirContent); err != nil {
		return nil, err
	}
	entry := dirContent.FindEntry(name)
	if entry == nil {
		return nil, errorx.New(errorx.KindNotFound, "nodeop: entry not found")
	}
	node, err := n.repo.Read(ctx, entry.NodeID)
	if err != nil {
		return nil, errorx.Wrap(err, "nodeop: read node")
	}
	return &fs.Dentry{Node: node, Name: entry.Name, Parent: dir}, nil
}
