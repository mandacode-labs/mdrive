package vfs

import (
	"context"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Ls lists the entries in a directory (like `ls /dir`). Permission
// is the caller's responsibility.
func (s *Service) Ls(ctx context.Context, driveID, path string) (node.DirContent, error) {
	logx.Debug(ctx, "vfs.ls.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
	)
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		logx.Debug(ctx, "vfs.ls.resolve_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return node.DirContent{}, err
	}
	if !res.Node.IsDir() {
		return node.DirContent{}, errorx.New(errorx.KindBadRequest, "vfs: not a directory")
	}
	dc, err := res.Node.ReadDir()
	if err != nil {
		logx.Debug(ctx, "vfs.ls.read_dir_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return node.DirContent{}, err
	}
	logx.Debug(ctx, "vfs.ls.ok",
		slog.String("drive_id", driveID),
		slog.Int("entry_count", len(dc.Entries)),
	)
	return dc, nil
}
