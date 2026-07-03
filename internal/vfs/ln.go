package vfs

import (
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Symlink creates a directory entry at linkPath pointing at
// target, matching `ln -s <target> <link>`. The target is
// recorded as a path string; the link may dangle.
//
// Permission is the caller's responsibility.
func (s *Service) Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error) {
	logx.Debug(ctx, "vfs.symlink.enter",
		slog.String("drive_id", driveID),
		slog.String("target", target),
		slog.String("link_path", linkPath),
	)
	parent, name, err := s.requireEditPath(ctx, driveID, linkPath)
	if err != nil {
		logx.Debug(ctx, "vfs.symlink.require_edit_path_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	n, err := node.NewSymlink(target)
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, n, parent, name); err != nil {
		logx.Debug(ctx, "vfs.symlink.create_link_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "vfs.symlink.ok",
		slog.String("drive_id", driveID),
		slog.String("inode_id", n.ID().String()),
	)
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
	logx.Debug(ctx, "vfs.hardlink.enter",
		slog.String("drive_id", driveID),
		slog.String("src_path", srcPath),
		slog.String("link_path", linkPath),
	)
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		logx.Debug(ctx, "vfs.hardlink.root_node_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, srcPath, true)
	if err != nil {
		logx.Debug(ctx, "vfs.hardlink.resolve_src_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	src := out.Node
	if src.IsDir() || src.IsMount() || src.IsSymlink() {
		logx.Debug(ctx, "vfs.hardlink.unsupported_type",
			slog.String("drive_id", driveID),
			slog.String("type", string(src.Type())),
		)
		return nil, ErrHardlinkNotSupported
	}
	parent, name, err := r.resolveParent(ctx, rootID, linkPath)
	if err != nil {
		logx.Debug(ctx, "vfs.hardlink.resolve_parent_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	if !parent.IsDir() {
		return nil, errorx.New(errorx.KindBadRequest, "vfs: not a directory")
	}
	if err := s.NodeClient.Link(ctx, parent, name, src); err != nil {
		logx.Debug(ctx, "vfs.hardlink.link_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "vfs.hardlink.ok", slog.String("drive_id", driveID))
	return src, nil
}

// ErrHardlinkNotSupported is returned when a hardlink source is
// of a type that POSIX does not allow to be hardlinked
// (directory, symlink, mount).
var ErrHardlinkNotSupported = errorx.New(errorx.KindBadRequest, "ln: source node type does not support hardlinks (dir/symlink/mount)")
