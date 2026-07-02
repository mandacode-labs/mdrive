package vfs

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Ls lists the entries in a directory (like `ls /dir`). Permission
// is the caller's responsibility.
func (s *Service) Ls(ctx context.Context, driveID, path string) (node.DirContent, error) {
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return node.DirContent{}, err
	}
	if !res.Node.IsDir() {
		return node.DirContent{}, errorx.New(errorx.KindBadRequest, "vfs: not a directory")
	}
	return res.Node.ReadDir()
}
