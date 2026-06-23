package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Mkdir creates a directory at path (like `mkdir /path/to/dir`).
func (s *Service) Mkdir(ctx context.Context, userID, driveID, path string) (*node.Node, error) {
	_, parent, name, err := s.requireEditPath(ctx, "mkdir", userID, driveID, path)
	if err != nil {
		return nil, err
	}
	dir, err := s.Node.CreateDirectory(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.createAndLink(ctx, "mkdir", dir, parent, name); err != nil {
		return nil, err
	}
	return dir, nil
}
