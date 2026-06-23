package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// LinkMode selects between the two POSIX `ln(1)` flavors.
type LinkMode bool

const (
	// Hardlink creates a new directory entry sharing the source
	// inode (POSIX ln <src> <link>). nlink is incremented on the
	// shared inode. The source must be a file or object node;
	// directories, symlinks, and mount nodes cannot be hardlinked.
	Hardlink LinkMode = false

	// Symlink creates a new symlink node pointing at the target
	// path (POSIX ln -s <target> <link>). The target is stored
	// as a path string; the link may dangle.
	Symlink LinkMode = true
)

// Ln creates a directory entry at linkPath pointing at target,
// matching the POSIX ln(1) command.
//
// When mode is Hardlink (the default; matching `ln`), target
// must be the path of an existing file or object node within
// driveID. The new entry shares the source inode and the
// source's nlink is incremented. Mount/symlink/directory
// sources are rejected with ErrHardlinkNotSupported.
//
// When mode is Symlink (matching `ln -s`), target is recorded
// as a path string. The target need not exist; dangling
// symlinks are allowed.
//
// Cross-drive hardlinks are rejected (POSIX requires same
// filesystem). For symlinks, target is opaque — vfs does not
// attempt to follow it.
//
// Permission is the caller's responsibility.
func (s *Service) Ln(ctx context.Context, driveID, target, linkPath string, mode LinkMode) (*node.Node, error) {
	if mode == Symlink {
		return s.lnSymlink(ctx, driveID, target, linkPath)
	}
	return s.lnHardlink(ctx, driveID, target, linkPath)
}

func (s *Service) lnSymlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error) {
	parent, name, err := s.requireEditPath(ctx, driveID, linkPath)
	if err != nil {
		return nil, err
	}
	n, err := s.Node.CreateSymlink(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, n, parent, name); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *Service) lnHardlink(ctx context.Context, driveID, srcPath, linkPath string) (*node.Node, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, srcPath)
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
	return "ln: source node type does not support hardlinks (dir/symlink/mount)"
}
