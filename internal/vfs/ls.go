package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Ls lists the entries in a directory (like `ls /dir`). Permission is
// checked on the drive the path ultimately resolves to.
func (s *Service) Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return node.DirContent{}, err
	}
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return node.DirContent{}, fmt.Errorf("ls: %w", err)
	}
	if res.DriveID != driveID {
		if err := s.checkAccess(ctx, userID, permission.PermissionView, res.DriveID); err != nil {
			return node.DirContent{}, fmt.Errorf("ls: %w", err)
		}
	}
	if !res.Node.IsDir() {
		return node.DirContent{}, ErrNotDirectory
	}
	return res.Node.ReadDir()
}
