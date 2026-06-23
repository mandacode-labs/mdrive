package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Ls lists the entries in a directory (like `ls /dir`). Permission is
// checked on the drive the path ultimately resolves to.
func (s *Service) Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error) {
	res, err := s.resolveView(ctx, "ls", userID, driveID, path)
	if err != nil {
		return node.DirContent{}, err
	}
	if !res.Node.IsDir() {
		return node.DirContent{}, ErrNotDirectory
	}
	return res.Node.ReadDir()
}
