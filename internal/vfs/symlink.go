package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Symlink creates a symbolic link at linkPath pointing to target (like `ln -s /target /link`).
func (s *Service) Symlink(ctx context.Context, userID, driveID, target, linkPath string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return nil, ErrNotFound
	}
	parent, name, err := s.path.resolveParent(ctx, *d.RootNodeID(), linkPath)
	if err != nil {
		return nil, fmt.Errorf("symlink: %w", err)
	}
	n, err := s.nodeSvc.CreateSymlink(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := s.nodeSvc.Link(ctx, parent, name, n); err != nil {
		_ = s.nodeSvc.Delete(ctx, n.ID())
		return nil, fmt.Errorf("symlink: link: %w", err)
	}
	return n, nil
}
