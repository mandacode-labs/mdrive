package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Ls lists the entries in a directory (like `ls /dir`). Permission
// is the caller's responsibility.
func (s *Service) Ls(ctx context.Context, driveID, path string) (node.DirContent, error) {
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return node.DirContent{}, errorx.Wrap(err, fmt.Sprintf("vfs: ls resolve (path=%s)", path))
	}
	if res.Node == nil {
		return node.DirContent{}, errorx.New(errorx.KindNotFound, "vfs: ls: not found (path="+path+")")
	}
	if !res.Node.IsDir() {
		return node.DirContent{}, errorx.New(errorx.KindBadRequest, "vfs: not a directory")
	}
	dc, err := res.Node.ReadDir()
	if err != nil {
		return node.DirContent{}, errorx.Wrap(err, fmt.Sprintf("vfs: ls read dir (path=%s)", path))
	}
	return dc, nil
}
