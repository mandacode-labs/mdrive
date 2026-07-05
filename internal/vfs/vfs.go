package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

type VFS interface {
	Lookup(ctx context.Context, dir *node.Node, name string) (*Dentry, error)
}

type vfs struct {
	nodeop  NodeOperation
	superop node.SuperOperation
}

func NewVFS(nodeop NodeOperation) VFS {
	return &vfs{
		nodeop: nodeop,
	}
}
