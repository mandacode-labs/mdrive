package vfs

import (
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Stat returns metadata for the file or directory at path. Permission
// is the caller's responsibility.
func (s *Service) Stat(ctx context.Context, driveID, path string) (*node.Node, error) {
	logx.Debug(ctx, "vfs.stat.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
	)
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		logx.Debug(ctx, "vfs.stat.resolve_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, err
	}
	logx.Debug(ctx, "vfs.stat.ok",
		slog.String("drive_id", driveID),
		slog.String("inode_id", res.Node.ID().String()),
	)
	return res.Node, nil
}
