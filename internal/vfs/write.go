package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Write creates or overwrites inline content at path.
func (s *Service) Write(ctx context.Context, userID, driveID, path, content string) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, path)
	if err != nil {
		// Path doesn't exist yet; treat the resolve error as a hint
		// to fall back to creating a new file. We need the parent
		// directory, so re-resolve with resolveParent.
		parent, name, perr := r.resolveParent(ctx, rootID, path)
		if perr != nil {
			return fmt.Errorf("write: %w", perr)
		}
		f, ferr := s.Node.CreateFile(ctx, content)
		if ferr != nil {
			return ferr
		}
		return s.createAndLink(ctx, "write", f, parent, name)
	}
	n := out.Node
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
	_, parent, name, err := s.requireEditPath(ctx, "write_large", userID, driveID, path)
	if err != nil {
		return err
	}
	n, err := s.Node.CreateObject(ctx, obj, size)
	if err != nil {
		return err
	}
	return s.createAndLink(ctx, "write_large", n, parent, name)
}
