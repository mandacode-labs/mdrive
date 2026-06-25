package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Symlink creates a directory entry at linkPath pointing at
// target, matching `ln -s <target> <link>`. The target is
// recorded as a path string; the link may dangle.
//
// Permission is the caller's responsibility.
func (s *Service) Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error) {
	parent, name, err := s.requireEditPath(ctx, driveID, linkPath)
	if err != nil {
		return nil, err
	}
	n, err := node.NewSymlink(target)
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, n, parent, name); err != nil {
		return nil, err
	}
	return n, nil
}

// Hardlink creates a new directory entry at linkPath sharing
// the inode at srcPath, matching `ln <src> <link>`. The
// source's nlink is incremented. Mount, symlink, and directory
// sources are rejected with ErrHardlinkNotSupported (POSIX
// only allows hardlinks to regular files; the additional
// restrictions on symlinks and mount nodes are mdrive-specific).
//
// Cross-drive hardlinks are rejected by the resolver (POSIX
// requires same filesystem).
//
// Permission is the caller's responsibility.
func (s *Service) Hardlink(ctx context.Context, driveID, srcPath, linkPath string) (*node.Node, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, srcPath, true)
	if err != nil {
		return nil, err
	}
	src := out.Node
	if src.IsDir() || src.IsMount() || src.IsSymlink() {
		return nil, ErrHardlinkNotSupported
	}
	parent, name, err := r.resolveParent(ctx, rootID, linkPath)
	if err != nil {
		return nil, err
	}
	if !parent.IsDir() {
		return nil, ErrNotDirectory
	}
	if err := s.NodeClient.Link(ctx, parent, name, src); err != nil {
		return nil, err
	}
	return src, nil
}

// ErrHardlinkNotSupported is returned when the source node type does
// not support hardlinks.
var ErrHardlinkNotSupported = &hardlinkError{}

type hardlinkError struct{}

func (*hardlinkError) Error() string {
	return "ln: source node type does not support hardlinks (dir/symlink/mount)"
}
