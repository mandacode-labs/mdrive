package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Ls lists the entries in a directory (like `ls /dir`).
func (s *Service) Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return node.DirContent{}, err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return node.DirContent{}, err
	}
	n, err := s.path.resolve(ctx, rootID, path)
	if err != nil {
		return node.DirContent{}, fmt.Errorf("ls: %w", err)
	}
	if !n.IsDir() {
		return node.DirContent{}, ErrNotDirectory
	}
	return n.ReadDir()
}
