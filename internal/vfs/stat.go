package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Stat returns metadata for the file or directory at path (like `stat /path`).
func (s *Service) Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return nil, err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return nil, ErrNotFound
	}
	n, err := s.path.resolve(ctx, *d.RootNodeID(), path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	return n, nil
}
