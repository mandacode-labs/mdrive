package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mount creates a bind mount of sourceDriveID at mountPath inside
// driveID's tree. The mount is a directory entry; resolving through it
// switches context to the source drive and continues with the remaining
// path. Permissions: caller must have edit on driveID (to create the
// mount entry) and view on sourceDriveID (to verify the source exists
// and is accessible).
//
// Same-drive mounts (driveID == sourceDriveID) are rejected to avoid
// trivial self-cycles.
func (s *Service) Mount(ctx context.Context, userID, driveID, mountPath, sourceDriveID string) (*node.Node, error) {
	if driveID == sourceDriveID {
		return nil, fmt.Errorf("mount: cannot mount a drive onto itself")
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionView, sourceDriveID); err != nil {
		return nil, fmt.Errorf("mount: source drive: %w", err)
	}
	// Validate source drive exists and has a root.
	src, err := s.Drive.GetByID(ctx, sourceDriveID)
	if err != nil {
		return nil, fmt.Errorf("mount: source drive lookup: %w", err)
	}
	if src == nil || src.RootNodeID() == nil {
		return nil, fmt.Errorf("mount: source drive has no root node")
	}

	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	parent, name, err := s.path.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return nil, fmt.Errorf("mount: resolve target: %w", err)
	}
	if !parent.IsDir() {
		return nil, ErrNotDirectory
	}
	mount, err := s.Node.CreateMount(ctx, sourceDriveID)
	if err != nil {
		return nil, err
	}
	if err := s.Node.Link(ctx, parent, name, mount); err != nil {
		_ = s.Node.Delete(ctx, mount.ID())
		return nil, fmt.Errorf("mount: link: %w", err)
	}
	return mount, nil
}

// Unmount removes the mount at mountPath within driveID. The mount
// node is deleted; the source drive and its data are untouched.
// Permissions: caller must have edit on driveID.
//
// If the entry at mountPath is not a mount, returns an error.
func (s *Service) Unmount(ctx context.Context, userID, driveID, mountPath string) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	out, err := s.path.resolve(ctx, rootID, mountPath)
	if err != nil {
		return fmt.Errorf("unmount: %w", err)
	}
	n := out.Node
	if !n.IsMount() {
		return fmt.Errorf("unmount: %s is not a mount", mountPath)
	}
	parent, name, err := s.path.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return fmt.Errorf("unmount: resolve parent: %w", err)
	}
	if _, err := s.Node.Unlink(ctx, parent, name); err != nil {
		return fmt.Errorf("unmount: unlink: %w", err)
	}
	return nil
}

