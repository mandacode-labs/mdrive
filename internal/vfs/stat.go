package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Stat returns metadata for the file or directory at path. Permission
// is checked on the drive the path ultimately resolves to.
func (s *Service) Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error) {
	res, err := s.resolveView(ctx, "stat", userID, driveID, path)
	if err != nil {
		return nil, err
	}
	return res.Node, nil
}
