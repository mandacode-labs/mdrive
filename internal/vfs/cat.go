package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Cat returns the inline bytes of a file node. The path is resolved
// with symlinks followed (POSIX cat(1) semantics). vfs is the inode
// tree manager and does not perform S3 I/O: for object nodes
// Cat returns ErrIsObject so the handler can route the request to
// the download/presign flow owned by upload.Service.
//
// Permission is the caller's responsibility.
func (s *Service) Cat(ctx context.Context, driveID, path string) ([]byte, error) {
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return nil, fmt.Errorf("cat: %w", err)
	}
	n := res.Node
	switch {
	case n.IsFile():
		raw, err := n.ReadFile()
		if err != nil {
			return nil, fmt.Errorf("cat: read: %w", err)
		}
		return []byte(raw), nil
	case n.IsObject():
		return nil, node.ErrIsObject
	case n.IsDir():
		return nil, node.ErrIsDirectory
	default:
		return nil, fmt.Errorf("cat: cannot read %s", n.Type())
	}
}
