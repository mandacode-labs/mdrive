package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// requireEditPath checks edit permission on driveID, fetches the drive's
// root node, and resolves path's parent directory. op is the calling
// operation name used in error wrapping. Returns ErrNotDirectory if the
// parent is not a directory.
func (s *Service) requireEditPath(ctx context.Context, op, userID, driveID, path string) (rootID uuid.UUID, parent *node.Node, name string, err error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return uuid.Nil, nil, "", fmt.Errorf("%s: %w", op, err)
	}
	rootID, err = s.rootNodeID(ctx, driveID)
	if err != nil {
		return uuid.Nil, nil, "", fmt.Errorf("%s: %w", op, err)
	}
	parent, name, err = s.newResolver().resolveParent(ctx, rootID, path)
	if err != nil {
		return uuid.Nil, nil, "", fmt.Errorf("%s: %w", op, err)
	}
	if !parent.IsDir() {
		return uuid.Nil, nil, "", fmt.Errorf("%s: %w", op, ErrNotDirectory)
	}
	return rootID, parent, name, nil
}

// resolveView checks view permission on driveID, resolves the path
// (transparently following mount nodes into other drives), and checks
// view on the resolved drive. This is the "look at a node, possibly
// through a mount" idiom: cross-drive traversal re-checks permissions
// on the source so a view on the parent does not implicitly grant
// access to the mounted subtree.
func (s *Service) resolveView(ctx context.Context, op, userID, driveID, path string) (Resolved, error) {
	if err := s.checkAccess(ctx, userID, permission.PermissionView, driveID); err != nil {
		return Resolved{}, fmt.Errorf("%s: %w", op, err)
	}
	res, err := s.Resolve(ctx, driveID, path)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: %w", op, err)
	}
	if res.DriveID != driveID {
		if err := s.checkAccess(ctx, userID, permission.PermissionView, res.DriveID); err != nil {
			return Resolved{}, fmt.Errorf("%s: %w", op, err)
		}
	}
	return res, nil
}

// createAndLink links child at parent/name. On link failure, the
// child is deleted to prevent leaking unparented inodes; the
// original link error is returned (cleanup error is logged via
// the returned wrapped error).
func (s *Service) createAndLink(ctx context.Context, op string, child, parent *node.Node, name string) error {
	if err := s.Node.Link(ctx, parent, name, child); err != nil {
		if derr := s.Node.Delete(ctx, child.ID()); derr != nil {
			return fmt.Errorf("%s: link: %w (cleanup: %v)", op, err, derr)
		}
		return fmt.Errorf("%s: link: %w", op, err)
	}
	return nil
}
