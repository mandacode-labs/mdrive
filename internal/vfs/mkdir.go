package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Mkdir creates a directory at path (like `mkdir /path/to/dir`).
// Permission is the caller's responsibility.
func (s *Service) Mkdir(ctx context.Context, driveID, path string) (*node.Node, error) {
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		return nil, err
	}
	dir, err := node.NewDirectory()
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, dir, parent, name); err != nil {
		return nil, err
	}
	return dir, nil
}
