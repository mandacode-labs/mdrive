package vfs

import (
	"context"
	"fmt"
)

// Cat reads the content of a file, symlink target, or object (like `cat /path`).
// Permission is checked on the drive the path ultimately resolves to, so
// a mount traversal into another drive requires view on the source.
func (s *Service) Cat(ctx context.Context, userID, driveID, path string) ([]byte, error) {
	res, err := s.resolveView(ctx, "cat", userID, driveID, path)
	if err != nil {
		return nil, err
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
