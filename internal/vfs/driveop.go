package vfs

import "context"

type DriveOperation interface {
	InitializeDrive(ctx context.Context, drive *Drive) error
	GetDrive(ctx context.Context, driveID string) (*Drive, error)
	DeleteDrive(ctx context.Context, drive *Drive) error
}
