package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Hardlink creates a new hardlink at linkPath pointing to the same node
// as srcPath, within driveID (POSIX ln <src> <link>).
//
// The new directory entry shares the inode of srcPath; the node's
// nlink is incremented. Cross-drive hardlinks are rejected (POSIX
// requires the source and target to be on the same filesystem).
func (s *Service) Hardlink(ctx context.Context, userID, driveID, srcPath, linkPath string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	out, err := s.path.resolve(ctx, rootID, srcPath)
	if err != nil {
		return nil, err
	}
	src := out.Node
	// Mounts, symlinks, directories cannot be hardlinked; only regular
	// files and object-backed files.
	if src.IsDir() || src.IsMount() || src.IsSymlink() {
		return nil, ErrHardlinkNotSupported
	}
	parent, name, err := s.path.resolveParent(ctx, rootID, linkPath)
	if err != nil {
		return nil, err
	}
	if !parent.IsDir() {
		return nil, ErrNotDirectory
	}
	if err := s.Node.Link(ctx, parent, name, src); err != nil {
		return nil, err
	}
	return src, nil
}

// ErrHardlinkNotSupported is returned when the source node type does
// not support hardlinks.
var ErrHardlinkNotSupported = &hardlinkError{}

type hardlinkError struct{}

func (*hardlinkError) Error() string {
	return "hardlink: source node type does not support hardlinks (dir/symlink/mount)"
}
