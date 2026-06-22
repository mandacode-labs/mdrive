package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Stat returns metadata for the file or directory at path. Permission
// is checked on the drive the path ultimately resolves to.
func (s *Service) Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return nil, err
	}
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if res.DriveID != driveID {
		if err := s.checkAccess(ctx, userID, permission.PermissionView, res.DriveID); err != nil {
			return nil, fmt.Errorf("stat: %w", err)
		}
	}
	return res.Node, nil
}
