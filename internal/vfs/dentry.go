package vfs

import "github.com/mandacode-labs/mdrive/internal/core/node"

type Dentry struct {
	Parent     *node.Node
	ParentName string
	Name       string
	Node       *node.Node
}
