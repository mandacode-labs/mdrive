package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mkdir creates a directory at path (like `mkdir /path/to/dir`).
func (s *Service) Mkdir(ctx context.Context, userID, driveID, path string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return nil, ErrNotFound
	}
	parent, name, err := s.path.resolveParent(ctx, *d.RootNodeID(), path)
	if err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if parent == nil || !parent.IsDir() {
		return nil, ErrNotDirectory
	}

	dir, err := s.nodeSvc.CreateDirectory(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.nodeSvc.Link(ctx, parent, name, dir); err != nil {
		_ = s.nodeSvc.Delete(ctx, dir.ID())
		return nil, fmt.Errorf("mkdir: link: %w", err)
	}
	return dir, nil
}
