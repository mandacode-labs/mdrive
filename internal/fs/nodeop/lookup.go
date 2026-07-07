package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Lookup resolves `name` under `parent` to a Dentry. The
// returned Dentry chains Parent = parent so callers can walk
// upward (e.g., for `..`).
func (n *nodeOperation) Lookup(ctx context.Context, parent *fs.Dentry, name string) (*fs.Dentry, error) {
	dirContent := &content.DirContent{}
	if err := json.Unmarshal(parent.Node.Data(), dirContent); err != nil {
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
	return &fs.Dentry{Node: node, Name: entry.Name, Parent: parent}, nil
}
