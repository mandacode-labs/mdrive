package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Lookup implements [NodeOperation].
func (n *nodeOperation) Lookup(ctx context.Context, dir *vfs.Node, name string) (*vfs.Dentry, error) {
	dirContent := &content.DirContent{}
	if err := json.Unmarshal(dir.Data(), dirContent); err != nil {
		return nil, err
	}
	entry := dirContent.FindEntry(name)
	if entry == nil {
		return nil, errorx.New(errorx.KindNotFound, "entry not found")
	}

	node, err := n.super.Read(ctx, entry.NodeID)
	if err != nil {
		return nil, errorx.Wrap(err, "failed to read node")
	}
	return &vfs.Dentry{
		Node:   node,
		Name:   entry.Name,
		Parent: dir,
	}, nil
}
