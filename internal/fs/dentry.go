package fs

import (
	"github.com/oklog/ulid/v2"
)

// Dentry is the result of a single walk step. Parent points
// at the parent dentry (matches Linux struct dentry::d_parent)
// so `..` traversal returns the parent directly.
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

// NewChildDentry constructs a child Dentry chained under parent.
func NewChildDentry(parent *Dentry, name string, node *Node) *Dentry {
	return &Dentry{DriveID: parent.DriveID, Parent: parent, Name: name, Node: node}
}

// NewMountRootDentry constructs the mount-source root dentry.
// mountPoint is the dentry at which the source is mounted; it
// lives in a different drive, so the parent chain crosses
// superblocks.
func NewMountRootDentry(sourceDriveID ulid.ULID, mountPoint *Dentry, root *Node) *Dentry {
	return &Dentry{DriveID: sourceDriveID, Parent: mountPoint, Name: "/", Node: root}
}
