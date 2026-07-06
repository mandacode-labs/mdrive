package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// SuperOperation is the per-drive root inode lookup surface vfs
// depends on. The full CRUD lives on superop.Repository.
type SuperOperation interface {
	Create(ctx context.Context, sb *Superblock) error
	Stat(ctx context.Context, id uuid.UUID) (*Superblock, error)
	GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error)

	// Purge removes the superblock and all its data. Destructive.
	Purge(ctx context.Context, id ulid.ULID) error
}
