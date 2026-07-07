package fs

import (
	"github.com/oklog/ulid/v2"
)

// Dentry is the result of a single walk step.
type Dentry struct {
	DriveID ulid.ULID
	Parent  *Node
	Name    string
	Node    *Node
}

// NewRootDentry constructs a Dentry for a drive's root inode.
func NewRootDentry(driveID ulid.ULID, root *Node) *Dentry {
	return &Dentry{DriveID: driveID, Parent: nil, Name: "/", Node: root}
}

// NewMountRootDentry constructs a Dentry for the root of a
// bind-mounted source drive.
func NewMountRootDentry(driveID ulid.ULID, root *Node) *Dentry {
	return &Dentry{DriveID: driveID, Parent: root, Name: "/", Node: root}
}
