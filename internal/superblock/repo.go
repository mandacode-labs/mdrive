package superblock

import (
	"context"

	"github.com/oklog/ulid/v2"
)

// Repository is the data-access contract for superblocks.
// Implemented by ent-backed repos; mocked in tests.
type Repository interface {
	Create(ctx context.Context, sb *Superblock) error
	GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error)
	DeleteByDriveID(ctx context.Context, driveID ulid.ULID) error
}