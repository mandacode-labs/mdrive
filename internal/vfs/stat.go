package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Stat returns metadata for the file or directory at path. Permission
// is the caller's responsibility.
func (s *service) Stat(ctx context.Context, driveID, path string) (*node.Node, error) {
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return nil, err
	}
	if res.Node == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: stat: not found (path="+path+")")
	}
	return res.Node, nil
}
