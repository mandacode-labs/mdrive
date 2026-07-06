package vfs

import (
	"context"
	"time"
)

// DriveOperation owns the Drive + Storage aggregate. Mirrors Linux
// super_operations plus drive CRUD. List queries are
// permission-agnostic; vfs's high-level wrappers enforce the
// caller is owner / is admin.
type DriveOperation interface {
	CreateDrive(ctx context.Context, ownerID, name, description string, storage *Storage) (*Drive, error)
	GetDrive(ctx context.Context, driveID string) (*Drive, error)
	GetDriveStorage(ctx context.Context, driveID string) (*Storage, error)
	UpdateDrive(ctx context.Context, driveID, name, description string) (*Drive, error)
	SoftDeleteDrive(ctx context.Context, driveID string) error
	RestoreDrive(ctx context.Context, driveID string) (*Drive, error)
	PurgeDrive(ctx context.Context, driveID string) error
	ListDrivesByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	ListDeletedDrives(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
}
