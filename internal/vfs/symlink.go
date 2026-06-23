package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Symlink creates a symbolic link at linkPath pointing to target (like `ln -s /target /link`).
// Permission is the caller's responsibility.
func (s *Service) Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error) {
	parent, name, err := s.requireEditPath(ctx, driveID, linkPath)
	if err != nil {
		return nil, err
	}
	n, err := s.Node.CreateSymlink(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, n, parent, name); err != nil {
		return nil, err
	}
	return n, nil
}
