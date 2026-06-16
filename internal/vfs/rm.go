package vfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Rm removes the file or directory at path (like `rm [-r] /path`).
// Set recursive=true to remove directories and their contents.
func (s *Service) Rm(ctx context.Context, userID, driveID, path string, recursive bool) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	d := s.mustGetDrive(ctx, driveID)
	if d == nil {
		return ErrNotFound
	}
	n, err := s.path.resolve(ctx, *d.RootNodeID(), path)
	if err != nil {
		return fmt.Errorf("rm: %w", err)
	}

	if n.IsDir() {
		if !recursive {
			return fmt.Errorf("rm: cannot remove '%s': is a directory (use -r)", path)
		}
		return s.rmRecursive(ctx, d, n, path)
	}
	return s.rmOne(ctx, d, n, path)
}

// rmOne removes a single file/symlink/object.
func (s *Service) rmOne(ctx context.Context, d *drive.Drive, n *node.Node, path string) error {
	parent, name, _ := s.path.resolveParent(ctx, *d.RootNodeID(), path)
	if parent != nil && name != "" {
		_ = s.nodeSvc.Unlink(ctx, parent, name)
	}
	if n.IsObject() {
		oc, _ := n.ReadObject()
		_ = s.store.DeleteObject(ctx, oc.Bucket, oc.Key)
	}
	return s.nodeSvc.Delete(ctx, n.ID())
}

// rmRecursive removes a directory tree depth-first.
func (s *Service) rmRecursive(ctx context.Context, d *drive.Drive, n *node.Node, path string) error {
	dc, err := n.ReadDir()
	if err != nil {
		return err
	}
	for _, e := range dc.Entries {
		childPath := strings.TrimRight(path, "/") + "/" + e.Name
		child, err := s.nodeSvc.GetByID(ctx, e.InodeID)
		if err != nil {
			return err
		}
		if child.IsDir() {
			if err := s.rmRecursive(ctx, d, child, childPath); err != nil {
				return err
			}
		} else {
			if err := s.rmOne(ctx, d, child, childPath); err != nil {
				return err
			}
		}
	}
	return s.rmOne(ctx, d, n, path)
}
