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
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	parent, name, err := s.newResolver().resolveParent(ctx, rootID, path)
	if err != nil {
		return nil, fmt.Errorf("touch: %w", err)
	}
	if !parent.IsDir() {
		return nil, ErrNotDirectory
	}
	n, err := s.Node.Touch(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.Node.Link(ctx, parent, name, n); err != nil {
		_ = s.Node.Delete(ctx, n.ID())
		return nil, fmt.Errorf("touch: link: %w", err)
	}
	return n, nil
}
