package vfs

import (
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Touch creates an empty file at path (like `touch /path`).
// Permission is the caller's responsibility.
func (s *Service) Touch(ctx context.Context, driveID, path string) (*node.Node, error) {
	logx.Debug(ctx, "vfs.touch.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
	)
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		logx.Debug(ctx, "vfs.touch.require_edit_path_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	n, err := node.NewFile("")
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, n, parent, name); err != nil {
		logx.Debug(ctx, "vfs.touch.create_link_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "vfs.touch.ok",
		slog.String("drive_id", driveID),
		slog.String("inode_id", n.ID().String()),
	)
	return n, nil
}
