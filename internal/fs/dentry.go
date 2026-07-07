package fs

import (
	"github.com/oklog/ulid/v2"
)

// Dentry is the result of a single walk step. Parent points
// at the parent dentry (matches Linux struct dentry::d_parent)
// so `..` traversal can return the parent directly without
// reconstructing it.
type Dentry struct {
	DriveID ulid.ULID
	Parent  *Dentry
	Name    string
	Node    *Node
}

// IsRoot reports whether this dentry is a drive root (no parent).
func (d *Dentry) IsRoot() bool { return d.Parent == nil }

// NewRootDentry constructs a Dentry for a drive's root inode.
func NewRootDentry(driveID ulid.ULID, root *Node) *Dentry {
	return &Dentry{DriveID: driveID, Parent: nil, Name: "/", Node: root}
}

// NewChildDentry constructs a Dentry for a child of `parent`.
// Inherits the parent's drive id; parent's Node becomes the
// Parent's parent (chain).
func NewChildDentry(parent *Dentry, name string, node *Node) *Dentry {
	return &Dentry{DriveID: parent.DriveID, Parent: parent, Name: name, Node: node}
}

// NewMountRootDentry constructs a Dentry for the root of a
// bind-mounted source drive. `mountPoint` is the dentry at
// which the source drive is mounted; it lives in a different
// drive, so its parent chain belongs to a different superblock.
func NewMountRootDentry(sourceDriveID ulid.ULID, mountPoint *Dentry, root *Node) *Dentry {
	return &Dentry{DriveID: sourceDriveID, Parent: mountPoint, Name: "/", Node: root}
}
