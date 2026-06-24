package vfs

import (
	"context"
	"fmt"
	"log/slog"

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
func (s *Service) Mount(ctx context.Context, driveID, mountPath, sourceDriveID string) error {
	if driveID == sourceDriveID {
		return fmt.Errorf("mount: cannot mount a drive onto itself")
	}
	// Validate source drive exists and has a root.
	src, err := s.Drive.GetByID(ctx, sourceDriveID)
	if err != nil {
		return fmt.Errorf("mount: source drive lookup: %w", err)
	}
	if src == nil || src.RootNodeID() == nil {
		return fmt.Errorf("mount: source drive has no root node")
	}
	// Reject soft-deleted source drives. Mounting a deleted drive
	// would let a caller reach data through a path that the rest
	// of the system treats as gone: the drive is hidden from
	// ListByOwner, restored only by an admin, and the
	// rootNodeID may be released when the soft-delete is
	// eventually hardened. Same check on the target drive is
	// the caller's responsibility: a mount can be removed by
	// the drive's owner (or admin via the cli) before they
	// soft-delete it, but mounting into a soft-deleted target
	// is a separate question that the handler layer should
	// gate.
	if src.DeletedAt() != nil {
		return fmt.Errorf("mount: source drive %s is soft-deleted", sourceDriveID)
	}

	parent, name, err := s.requireEditPath(ctx, driveID, mountPath)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	mount, err := node.NewMount(sourceDriveID)
	if err != nil {
		return err
	}
	if err := s.createAndLink(ctx, mount, parent, name); err != nil {
		s.log().Debug("vfs.mount.failed",
			slog.String("from_drive", driveID),
			slog.String("to_drive", sourceDriveID),
			slog.String("path", mountPath),
			slog.String("err", err.Error()),
		)
		return fmt.Errorf("mount: %w", err)
	}
	s.log().Info("vfs.mount.created",
		slog.String("from_drive", driveID),
		slog.String("to_drive", sourceDriveID),
		slog.String("path", mountPath),
	)
	return nil
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
	srcDrive, _ := n.ReadMount()
	parent, name, err := r.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return fmt.Errorf("unmount: resolve parent: %w", err)
	}
	if _, err := s.Node.Unlink(ctx, parent, name); err != nil {
		s.log().Debug("vfs.unmount.failed",
			slog.String("drive_id", driveID),
			slog.String("path", mountPath),
			slog.String("err", err.Error()),
		)
		return fmt.Errorf("unmount: unlink: %w", err)
	}
	s.log().Info("vfs.unmount.completed",
		slog.String("drive_id", driveID),
		slog.String("path", mountPath),
		slog.String("source_drive", srcDrive),
	)
	return nil
}
