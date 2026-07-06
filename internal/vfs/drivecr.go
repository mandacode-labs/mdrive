package vfs

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// CreateDrive persists a new Drive + Storage. Permission is
// the caller's responsibility (the vfs high-level wrappers
// gate by ownerID == caller). DriveOperation owns the
// implementation; this is a thin pass-through.
func (v *vfs) CreateDrive(ctx context.Context, ownerID string, name string, description string, storage *Storage) (*Drive, error) {
	return v.driveOp.CreateDrive(ctx, ownerID, name, description, storage)
}

// GetDrive fetches a drive by id.
func (v *vfs) GetDrive(ctx context.Context, driveID string) (*Drive, error) {
	return v.driveOp.GetDrive(ctx, driveID)
}

// GetDriveStorage fetches the per-drive storage configuration.
func (v *vfs) GetDriveStorage(ctx context.Context, driveID string) (*Storage, error) {
	return v.driveOp.GetDriveStorage(ctx, driveID)
}

// UpdateDrive updates name and/or description.
func (v *vfs) UpdateDrive(ctx context.Context, driveID string, name string, description string) (*Drive, error) {
	return v.driveOp.UpdateDrive(ctx, driveID, name, description)
}

// SoftDeleteDrive marks the drive as deleted. Reversible via
// RestoreDrive.
func (v *vfs) SoftDeleteDrive(ctx context.Context, driveID string) error {
	return v.driveOp.SoftDeleteDrive(ctx, driveID)
}

// RestoreDrive clears the soft-delete timestamp.
func (v *vfs) RestoreDrive(ctx context.Context, driveID string) (*Drive, error) {
	return v.driveOp.RestoreDrive(ctx, driveID)
}

// PurgeDrive hard-deletes the drive; cascade on Storage fires.
func (v *vfs) PurgeDrive(ctx context.Context, driveID string) error {
	return v.driveOp.PurgeDrive(ctx, driveID)
}

// ListDrives returns active drives owned by userID. vfs
// enforces that the caller equals userID.
func (v *vfs) ListDrives(ctx context.Context, userID string) ([]*Drive, error) {
	if v.userID(ctx) != userID {
		return nil, errorx.New(errorx.KindPermissionDenied, "vfs: only the owner may list their drives")
	}
	return v.driveOp.ListDrivesByOwner(ctx, userID)
}

// ListDeletedDrives returns soft-deleted drives. vfs enforces
// that the caller is admin.
func (v *vfs) ListDeletedDrives(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	if !v.isAdmin(ctx) {
		return nil, errorx.New(errorx.KindPermissionDenied, "vfs: only admins may list deleted drives")
	}
	return v.driveOp.ListDeletedDrives(ctx, before, limit)
}

// silence unused ulid import check.
var _ = ulid.ULID{}
