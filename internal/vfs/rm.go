package vfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Rm removes the file or directory at path (like `rm [-r] /path`).
func (s *Service) Rm(ctx context.Context, userID, driveID, path string, recursive bool) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	n, err := s.path.resolve(ctx, rootID, path)
	if err != nil {
		return fmt.Errorf("rm: %w", err)
	}
	if n.IsDir() {
		if !recursive {
			return fmt.Errorf("rm: '%s' is a directory (use -r)", path)
		}
		return s.rmRecursive(ctx, rootID, n, path)
	}
	return s.rmOne(ctx, rootID, n, path)
}

func (s *Service) rmOne(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) error {
	parent, name, _ := s.path.resolveParent(ctx, rootID, path)
	if parent != nil && name != "" {
		_ = s.node.Unlink(ctx, parent, name)
	}
	if n.IsObject() {
		oc, _ := n.ReadObject()
		_ = s.store.DeleteObject(ctx, oc.Bucket, oc.Key)
	}
	return s.node.Delete(ctx, n.ID())
}

func (s *Service) rmRecursive(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) error {
	dc, err := n.ReadDir()
	if err != nil {
		return err
	}
	for _, e := range dc.Entries {
		childPath := strings.TrimRight(path, "/") + "/" + e.Name
		child, err := s.node.GetByID(ctx, e.InodeID)
		if err != nil {
			return err
		}
		if child.IsDir() {
			if err := s.rmRecursive(ctx, rootID, child, childPath); err != nil {
				return err
			}
		} else {
			if err := s.rmOne(ctx, rootID, child, childPath); err != nil {
				return err
			}
		}
	}
	return s.rmOne(ctx, rootID, n, path)
}
