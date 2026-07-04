package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Mkdir creates a directory at path (like `mkdir /path/to/dir`).
// Permission is the caller's responsibility.
func (s *service) Mkdir(ctx context.Context, driveID, path string) (*node.Node, error) {
	parent, name, err := s.resolveEditableParent(ctx, driveID, path)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mkdir resolve parent (drive_id=%s, path=%s)", driveID, path))
	}
	if parent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mkdir parent not found (drive_id="+driveID+", path="+path+")")
	}
	dir, err := node.NewDirectory()
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mkdir new directory (drive_id=%s, path=%s)", driveID, path))
	}
	if err := s.NodeClient.Link(ctx, parent, name, dir); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mkdir link (drive_id=%s, path=%s)", driveID, path))
	}
	return dir, nil
}
