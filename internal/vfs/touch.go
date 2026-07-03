package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Touch creates an empty file at path (like `touch /path`).
// Permission is the caller's responsibility.
func (s *Service) Touch(ctx context.Context, driveID, path string) (*node.Node, error) {
	parent, name, err := s.resolveEditableParent(ctx, driveID, path)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: touch resolve parent (drive_id=%s, path=%s)", driveID, path))
	}
	if parent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: touch parent not found (drive_id="+driveID+", path="+path+")")
	}
	n, err := node.NewFile("")
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: touch new file (drive_id=%s, path=%s)", driveID, path))
	}
	if err := s.NodeClient.Link(ctx, parent, name, n); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: touch link (drive_id=%s, path=%s)", driveID, path))
	}
	return n, nil
}
