package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Mount creates a bind mount of sourceDriveID at mountPath inside
// driveID's tree. The mount is a directory entry; resolving through it
// switches context to the source drive and continues with the remaining
// path.
//
// Same-drive mounts (driveID == sourceDriveID) are rejected to avoid
// trivial self-cycles.
//
// Permission is the caller's responsibility: edit on driveID (to
// create the mount entry) and view on sourceDriveID (to verify the
// source exists and is accessible).
func (s *Service) Mount(ctx context.Context, driveID, mountPath, sourceDriveID string) (*node.Node, error) {
	if driveID == sourceDriveID {
		return nil, fmt.Errorf("mount: cannot mount a drive onto itself")
	}
	// Validate source drive exists and has a root.
	src, err := s.Drive.GetByID(ctx, sourceDriveID)
	if err != nil {
		return nil, fmt.Errorf("mount: source drive lookup: %w", err)
	}
	if src == nil || src.RootNodeID() == nil {
		return nil, fmt.Errorf("mount: source drive has no root node")
	}

	parent, name, err := s.requireEditPath(ctx, driveID, mountPath)
	if err != nil {
		return nil, fmt.Errorf("mount: %w", err)
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
//
// If the entry at mountPath is not a mount, returns an error.
//
// Permission is the caller's responsibility.
func (s *Service) Unmount(ctx context.Context, driveID, mountPath string) error {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	// Use a single resolver instance so the resolve + resolveParent
	// pair see the same *Node pointer for any shared intermediate
	// nodes.
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, mountPath, true)
	if err != nil {
		return fmt.Errorf("unmount: %w", err)
	}
	n := out.Node
	if !n.IsMount() {
		return fmt.Errorf("unmount: %s is not a mount", mountPath)
	}
	parent, name, err := r.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return fmt.Errorf("unmount: resolve parent: %w", err)
	}
	if _, err := s.Node.Unlink(ctx, parent, name); err != nil {
		return fmt.Errorf("unmount: unlink: %w", err)
	}
	return nil
}
