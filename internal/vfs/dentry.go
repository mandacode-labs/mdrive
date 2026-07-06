package vfs

// Dentry is the result of a single walk step: the parent
// directory, the entry name within it, and the inode at rest.
type Dentry struct {
	Parent *Node
	Name   string
	Node   *Node
}
