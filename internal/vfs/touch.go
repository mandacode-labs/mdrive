package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Touch creates an empty file at path (like `touch /path`).
func (s *Service) Touch(ctx context.Context, userID, driveID, path string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return nil, ErrNotFound
	}
	parent, name, err := s.path.resolveParent(ctx, *d.RootNodeID(), path)
	if err != nil {
		return nil, fmt.Errorf("touch: %w", err)
	}
	if parent == nil || !parent.IsDir() {
		return nil, ErrNotDirectory
	}

	n, err := s.nodeSvc.CreateFile(ctx, "")
	if err != nil {
		return nil, err
	}
	if err := s.nodeSvc.Link(ctx, parent, name, n); err != nil {
		_ = s.nodeSvc.Delete(ctx, n.ID())
		return nil, fmt.Errorf("touch: link: %w", err)
	}
	return n, nil
}
