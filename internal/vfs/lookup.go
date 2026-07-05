package vfs

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Lookup implements [NodeOperation].
func (n *vfs) Lookup(ctx context.Context, dir *node.Node, name string) (*Dentry, error) {
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
	return &Dentry{
		Node:   node,
		Name:   entry.Name,
		Parent: dir,
	}, nil
}
