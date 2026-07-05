package vfs

import "context"

type DriveOperation interface {
	Initialize(ctx context.Context, drive *Drive) error
	Get(ctx context.Context, driveID string) (*Drive, error)
	Delete(ctx context.Context, drive *Drive) error
}
