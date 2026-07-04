package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Symlink creates a directory entry at linkPath pointing at
// target, matching `ln -s <target> <link>`. The target is
// recorded as a path string; the link may dangle.
//
// Permission is the caller's responsibility.
func (s *service) Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error) {
	parent, name, err := s.resolveEditableParent(ctx, driveID, linkPath)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: symlink resolve parent (drive_id=%s, link_path=%s)", driveID, linkPath))
	}
	if parent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: symlink parent not found (drive_id="+driveID+", link_path="+linkPath+")")
	}
	n, err := node.NewSymlink(target)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: symlink new (drive_id=%s, target=%s)", driveID, target))
	}
	if err := s.NodeClient.Link(ctx, parent, name, n); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: symlink link (drive_id=%s, link_path=%s)", driveID, linkPath))
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
func (s *service) Hardlink(ctx context.Context, driveID, srcPath, linkPath string) (*node.Node, error) {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := newResolver(s.NodeClient)
	out, err := r.resolvePath(ctx, rootID, srcPath, true)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: hardlink resolve src (drive_id=%s, src_path=%s)", driveID, srcPath))
	}
	if out.Node == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: hardlink src not found (drive_id="+driveID+", src_path="+srcPath+")")
	}
	src := out.Node
	if src.IsDir() || src.IsMount() || src.IsSymlink() {
		return nil, ErrHardlinkNotSupported
	}
	parent, name, err := r.resolveParent(ctx, rootID, linkPath)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: hardlink resolve parent (drive_id=%s, link_path=%s)", driveID, linkPath))
	}
	if parent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: hardlink parent not found (drive_id="+driveID+", link_path="+linkPath+")")
	}
	if !parent.IsDir() {
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: not a directory")
	}
	if err := s.NodeClient.Link(ctx, parent, name, src); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: hardlink link (drive_id=%s, link_path=%s)", driveID, linkPath))
	}
	return src, nil
}

// ErrHardlinkNotSupported is returned when a hardlink source is
// of a type that POSIX does not allow to be hardlinked
// (directory, symlink, mount).
var ErrHardlinkNotSupported = errorx.New(errorx.KindInvalidArgument, "ln: source node type does not support hardlinks (dir/symlink/mount)")
