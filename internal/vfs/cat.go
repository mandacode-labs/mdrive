package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Cat reads the content of a file, symlink target, or object (like `cat /path`).
// For object nodes, fetches the actual data from S3 transparently.
func (s *Service) Cat(ctx context.Context, userID, driveID, path string) ([]byte, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return nil, err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return nil, ErrNotFound
	}
	n, err := s.path.resolve(ctx, *d.RootNodeID(), path)
	if err != nil {
		return nil, fmt.Errorf("cat: %w", err)
	}
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
		data, err := s.store.GetObject(ctx, oc.Bucket, oc.Key)
		if err != nil {
			return nil, fmt.Errorf("cat: store: %w", err)
		}
		return data, nil
	case n.IsSymlink():
		target, err := n.ReadSymlink()
		if err != nil {
			return nil, err
		}
		return []byte(target), nil
	default:
		return nil, fmt.Errorf("cat: cannot read %s", n.Type())
	}
}

// Write writes inline content to a file at path (4KB max).
// Creates the file if it doesn't exist.
func (s *Service) Write(ctx context.Context, userID, driveID, path, content string) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return ErrNotFound
	}
	// Try to resolve existing node; if not found, create one.
	n, err := s.path.resolve(ctx, *d.RootNodeID(), path)
	if err != nil {
		parent, name, perr := s.path.resolveParent(ctx, *d.RootNodeID(), path)
		if perr != nil {
			return fmt.Errorf("write: %w", perr)
		}
		f, ferr := s.nodeSvc.CreateFile(ctx, content)
		if ferr != nil {
			return ferr
		}
		if lerr := s.nodeSvc.Link(ctx, parent, name, f); lerr != nil {
			_ = s.nodeSvc.Delete(ctx, f.ID())
			return fmt.Errorf("write: link: %w", lerr)
		}
		return nil
	}
	if !n.IsFile() {
		return fmt.Errorf("write: cannot write to %s", n.Type())
	}
	return n.WriteFile(content)
}

// WriteLarge creates or updates an object (S3-backed) node at path.
// The actual S3 upload happens separately; this just records the reference.
func (s *Service) WriteLarge(ctx context.Context, userID, driveID, path string, obj node.ObjectContent, size int64) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return ErrNotFound
	}
	parent, name, perr := s.path.resolveParent(ctx, *d.RootNodeID(), path)
	if perr != nil {
		return fmt.Errorf("write_large: %w", perr)
	}
	n, err := s.nodeSvc.CreateObject(ctx, obj, size)
	if err != nil {
		return err
	}
	if lerr := s.nodeSvc.Link(ctx, parent, name, n); lerr != nil {
		_ = s.nodeSvc.Delete(ctx, n.ID())
		return fmt.Errorf("write_large: link: %w", lerr)
	}
	return nil
}
