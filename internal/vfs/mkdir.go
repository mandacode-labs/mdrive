package vfs

import (
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Mkdir creates a directory at path (like `mkdir /path/to/dir`).
// Permission is the caller's responsibility.
func (s *Service) Mkdir(ctx context.Context, driveID, path string) (*node.Node, error) {
	logx.Debug(ctx, "vfs.mkdir.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
	)
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		logx.Debug(ctx, "vfs.mkdir.require_edit_path_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	dir, err := node.NewDirectory()
	if err != nil {
		logx.Debug(ctx, "vfs.mkdir.new_dir_err", slog.String("err", err.Error()))
		return nil, err
	}
	if err := s.createAndLink(ctx, dir, parent, name); err != nil {
		logx.Debug(ctx, "vfs.mkdir.create_link_err",
			slog.String("drive_id", driveID),
			slog.String("path", path),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "vfs.mkdir.ok",
		slog.String("drive_id", driveID),
		slog.String("path", path),
		slog.String("inode_id", dir.ID().String()),
	)
	return dir, nil
}
