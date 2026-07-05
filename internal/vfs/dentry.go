package vfs

type Dentry struct {
	Parent     *Node
	ParentName string
	Name       string
	Node       *Node
}
