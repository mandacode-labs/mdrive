package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// SuperOperation
type SuperOperation interface {
	Create(ctx context.Context, sb *Superblock) error
	Stat(ctx context.Context, id uuid.UUID) (*Superblock, error)
	GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error)
	// Danger: Purge is a destructive operation that removes the superblock and all associated data. Use with caution.
	Purge(ctx context.Context, id ulid.ULID) error
}
