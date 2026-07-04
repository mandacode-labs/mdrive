package vfs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
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
func (s *service) Mount(ctx context.Context, driveID, mountPath, sourceDriveID string) error {
	if driveID == sourceDriveID {
		return errorx.New(errorx.KindInvalidArgument, "vfs: mount self-cycle (drive_id="+driveID+")")
	}
	src, err := s.DriveClient.GetByID(ctx, sourceDriveID)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: mount source drive lookup (source_drive=%s)", sourceDriveID))
	}
	if src == nil || src.RootNodeID() == nil {
		return errorx.New(errorx.KindNotFound, "vfs: mount source drive has no root (source_drive="+sourceDriveID+")")
	}
	if src.DeletedAt() != nil {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: mount source drive is soft-deleted (source_drive="+sourceDriveID+")")
	}

	parent, name, err := s.resolveEditableParent(ctx, driveID, mountPath)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: mount resolve parent (drive_id=%s, mount_path=%s)", driveID, mountPath))
	}
	if parent == nil {
		return errorx.New(errorx.KindNotFound, "vfs: mount parent not found (drive_id="+driveID+", mount_path="+mountPath+")")
	}
	mount, err := node.NewMount(sourceDriveID)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: mount new (drive_id=%s, source_drive=%s)", driveID, sourceDriveID))
	}
	if err := s.NodeClient.Link(ctx, parent, name, mount); err != nil {
		logx.Debug(ctx, "vfs.mount.failed",
			slog.String("from_drive", driveID),
			slog.String("to_drive", sourceDriveID),
			slog.String("path", mountPath),
			slog.String("err", err.Error()),
		)
		return errorx.Wrap(err, fmt.Sprintf("vfs: mount link (drive_id=%s, mount_path=%s)", driveID, mountPath))
	}
	logx.Info(ctx, "vfs.mount.created",
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
func (s *service) Unmount(ctx context.Context, driveID, mountPath string) error {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	r := newResolver(s.NodeClient)
	out, err := r.resolvePath(ctx, rootID, mountPath, true)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: unmount resolve (drive_id=%s, mount_path=%s)", driveID, mountPath))
	}
	if out.Node == nil {
		return errorx.New(errorx.KindNotFound, "vfs: unmount target not found (drive_id="+driveID+", mount_path="+mountPath+")")
	}
	n := out.Node
	if !n.IsMount() {
		return errorx.New(errorx.KindFailedPrecondition, "vfs: unmount target is not a mount (drive_id="+driveID+", mount_path="+mountPath+")")
	}
	srcDrive, err := n.ReadMount()
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: unmount read mount (drive_id=%s, mount_path=%s)", driveID, mountPath))
	}
	parent, name, err := r.resolveParent(ctx, rootID, mountPath)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: unmount resolve parent (drive_id=%s, mount_path=%s)", driveID, mountPath))
	}
	if parent == nil {
		return errorx.New(errorx.KindNotFound, "vfs: unmount parent not found (drive_id="+driveID+", mount_path="+mountPath+")")
	}
	if _, err := s.NodeClient.Unlink(ctx, parent, name); err != nil {
		logx.Debug(ctx, "vfs.unmount.failed",
			slog.String("drive_id", driveID),
			slog.String("path", mountPath),
			slog.String("err", err.Error()),
		)
		return errorx.Wrap(err, fmt.Sprintf("vfs: unmount unlink (drive_id=%s, mount_path=%s)", driveID, mountPath))
	}
	logx.Info(ctx, "vfs.unmount.completed",
		slog.String("drive_id", driveID),
		slog.String("path", mountPath),
		slog.String("source_drive", srcDrive),
	)
	return nil
}
