package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Cat returns the inline bytes of a file node. The path is resolved
// with symlinks followed (POSIX cat(1) semantics). vfs is the inode
// tree manager and does not perform S3 I/O: for object nodes
// Cat returns ErrIsObject so the handler can route the request to
// the download/presign flow owned by upload.Service.
//
// Permission is the caller's responsibility.
func (s *service) Cat(ctx context.Context, driveID, path string) ([]byte, error) {
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: cat resolve (path=%s)", path))
	}
	if res.Node == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: cat: not found (path="+path+")")
	}
	n := res.Node
	switch {
	case n.IsFile():
		raw, err := n.ReadFile()
		if err != nil {
			return nil, errorx.Wrap(err, fmt.Sprintf("vfs: cat read file (path=%s)", path))
		}
		return []byte(raw), nil
	case n.IsObject():
		return nil, errorx.New(errorx.KindFailedPrecondition, "vfs: cat: target is an object (use download endpoint)")
	case n.IsDir():
		return nil, errorx.New(errorx.KindInvalidArgument, "vfs: cat: target is a directory")
	default:
		return nil, errorx.New(errorx.KindFailedPrecondition, "vfs: cat: cannot read type="+string(n.Kind()))
	}
}
