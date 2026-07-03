package vfs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Cat returns the inline bytes of a file node. The path is resolved
// with symlinks followed (POSIX cat(1) semantics). vfs is the inode
// tree manager and does not perform S3 I/O: for object nodes
// Cat returns ErrIsObject so the handler can route the request to
// the download/presign flow owned by upload.Service.
//
// Permission is the caller's responsibility.
func (s *Service) Cat(ctx context.Context, driveID, path string) ([]byte, error) {
	logx.Debug(ctx, "vfs.cat.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
	)
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		logx.Debug(ctx, "vfs.cat.resolve_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: cat resolve (path=%s)", path))
	}
	n := res.Node
	switch {
	case n.IsFile():
		raw, err := n.ReadFile()
		if err != nil {
			logx.Debug(ctx, "vfs.cat.read_file_err",
				slog.String("drive_id", driveID),
				slog.String("err", err.Error()),
			)
			return nil, errorx.Wrap(err, fmt.Sprintf("vfs: cat read file (path=%s)", path))
		}
		logx.Debug(ctx, "vfs.cat.ok",
			slog.String("drive_id", driveID),
			slog.Int("bytes", len(raw)),
		)
		return []byte(raw), nil
	case n.IsObject():
		return nil, errorx.New(errorx.KindBadRequest, "vfs: cat: target is an object")
	case n.IsDir():
		return nil, errorx.New(errorx.KindBadRequest, "vfs: cat: target is a directory")
	default:
		return nil, errorx.New(errorx.KindBadRequest, "vfs: cat: cannot read type="+string(n.Kind()))
	}
}
