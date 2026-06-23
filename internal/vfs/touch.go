package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Touch creates an empty file at path (like `touch /path`).
// Permission is the caller's responsibility.
func (s *Service) Touch(ctx context.Context, driveID, path string) (*node.Node, error) {
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		return nil, err
	}
	n, err := s.Node.Touch(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, n, parent, name); err != nil {
		return nil, err
	}
	return n, nil
}
