package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Stat returns metadata for the file or directory at path. Permission
// is the caller's responsibility.
func (s *Service) Stat(ctx context.Context, driveID, path string) (*node.Node, error) {
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return nil, err
	}
	return res.Node, nil
}
