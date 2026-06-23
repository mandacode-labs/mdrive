package vfs

import (
	"context"
	"fmt"
)

// Cat reads the content of a file, symlink target, or object (like `cat /path`).
// Permission is the caller's responsibility: vfs does not check.
// The caller should have already verified view permission on the
// drive the path ultimately resolves to.
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
		oc, err := n.ReadObject()
		if err != nil {
			return nil, err
		}
		data, err := s.Store.GetObject(ctx, oc.Bucket, oc.Key)
		if err != nil {
			return nil, fmt.Errorf("cat: store: %w", err)
		}
		return data, nil
	case n.IsSymlink():
		target, err := n.ReadSymlink()
		if err != nil {
			return nil, fmt.Errorf("cat: read: %w", err)
		}
		return []byte(target), nil
	default:
		return nil, fmt.Errorf("cat: cannot read %s", n.Type())
	}
}
