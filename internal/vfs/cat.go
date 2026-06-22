package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Cat reads the content of a file, symlink target, or object (like `cat /path`).
// Permission is checked on the drive the path ultimately resolves to, so
// a mount traversal into another drive requires view on the source.
func (s *Service) Cat(ctx context.Context, userID, driveID, path string) ([]byte, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return nil, err
	}
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return nil, fmt.Errorf("cat: %w", err)
	}
	if res.DriveID != driveID {
		if err := s.checkAccess(ctx, userID, permission.PermissionView, res.DriveID); err != nil {
			return nil, fmt.Errorf("cat: %w", err)
		}
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

// Write creates or overwrites inline content at path.
func (s *Service) Write(ctx context.Context, userID, driveID, path, content string) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	n, err := s.path.resolve(ctx, rootID, path)
	if err != nil {
		parent, name, perr := s.path.resolveParent(ctx, rootID, path)
		if perr != nil {
			return fmt.Errorf("write: %w", perr)
		}
		f, ferr := s.Node.CreateFile(ctx, content)
		if ferr != nil {
			return ferr
		}
		if lerr := s.Node.Link(ctx, parent, name, f); lerr != nil {
			if derr := s.Node.Delete(ctx, f.ID()); derr != nil {
				return fmt.Errorf("write: link: %w (cleanup: %v)", lerr, derr)
			}
			return fmt.Errorf("write: link: %w", lerr)
		}
		return nil
	}
	if !n.IsFile() {
		return fmt.Errorf("write: cannot write to %s", n.Type())
	}
	if err := n.WriteFile(content); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return s.Node.Save(ctx, n)
}

// WriteLarge creates an object (S3-backed) node at path.
func (s *Service) WriteLarge(ctx context.Context, userID, driveID, path string, obj node.ObjectContent, size int64) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	// Check if path already exists
	if n, err := s.path.resolve(ctx, rootID, path); err == nil {
		return fmt.Errorf("write_large: %s: already exists (type=%s)", path, n.Type())
	}
	parent, name, perr := s.path.resolveParent(ctx, rootID, path)
	if perr != nil {
		return fmt.Errorf("write_large: %w", perr)
	}
	n, err := s.Node.CreateObject(ctx, obj, size)
	if err != nil {
		return err
	}
	if lerr := s.Node.Link(ctx, parent, name, n); lerr != nil {
		if derr := s.Node.Delete(ctx, n.ID()); derr != nil {
			return fmt.Errorf("write_large: link: %w (cleanup: %v)", lerr, derr)
		}
		return fmt.Errorf("write_large: link: %w", lerr)
	}
	return nil
}
