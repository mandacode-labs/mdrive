package vfs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Write creates or overwrites inline content at path.
// Permission is the caller's responsibility.
//
// The create-on-missing branch constructs the file in memory
// and hands it to Node.Link, which inserts the row inside the
// same transaction as the parent's directory update. The
// overwrite branch goes through Node.Save, whose UPDATE is
// committed atomically.
func (s *Service) Write(ctx context.Context, driveID, path, content string) error {
	logx.Debug(ctx, "vfs.write.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
		slog.Int("content_len", len(content)),
	)
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		logx.Debug(ctx, "vfs.write.root_node_err",
			slog.String("drive_id", driveID),
			slog.String("err", err.Error()),
		)
		return err
	}
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, path, true)
	if err != nil {
		logx.Debug(ctx, "vfs.write.resolve_err_falling_back_to_create",
			slog.String("drive_id", driveID),
			slog.String("path", path),
			slog.String("err", err.Error()),
		)
		parent, name, perr := r.resolveParent(ctx, rootID, path)
		if perr != nil {
			return errorx.Wrap(perr, fmt.Sprintf("vfs: write resolve parent (path=%s)", path))
		}
		f, ferr := node.NewFile(content)
		if ferr != nil {
			return ferr
		}
		return s.createAndLink(ctx, f, parent, name)
	}
	n := out.Node
	if !n.IsFile() {
		return errorx.New(errorx.KindBadRequest, "vfs: write target is not a file (type="+string(n.Kind())+")")
	}
	if err := n.WriteFile(content); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: write encode content (path=%s)", path))
	}
	return s.NodeClient.Save(ctx, n)
}

// WriteLarge creates an object (S3-backed) node at path.
// Permission is the caller's responsibility.
func (s *Service) WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error {
	logx.Debug(ctx, "vfs.write_large.enter",
		slog.String("drive_id", driveID),
		slog.String("path", path),
		slog.String("bucket", obj.Bucket),
		slog.Int64("size", size),
	)
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		return err
	}
	n, err := node.NewObject(obj, size)
	if err != nil {
		return err
	}
	return s.createAndLink(ctx, n, parent, name)
}
