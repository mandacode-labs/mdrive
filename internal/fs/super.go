package fs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Superblock is the per-drive inode carrier. Mirrors Linux
// struct super_block.
type Superblock struct {
	id         uuid.UUID
	driveID    ulid.ULID
	rootNodeID uuid.UUID
	createdAt  time.Time
	updatedAt  time.Time
}

func (s *Superblock) ID() uuid.UUID         { return s.id }
func (s *Superblock) DriveID() ulid.ULID    { return s.driveID }
func (s *Superblock) RootNodeID() uuid.UUID { return s.rootNodeID }
func (s *Superblock) CreatedAt() time.Time  { return s.createdAt }
func (s *Superblock) UpdatedAt() time.Time  { return s.updatedAt }

// NewSuperblock constructs a fresh Superblock.
func NewSuperblock(driveID ulid.ULID, rootNodeID uuid.UUID) *Superblock {
	now := time.Now()
	return &Superblock{
		id:         uuid.New(),
		driveID:    driveID,
		rootNodeID: rootNodeID,
		createdAt:  now,
		updatedAt:  now,
	}
}

// HydrateSuperblock reconstructs a Superblock from persisted fields.
func HydrateSuperblock(
	id uuid.UUID,
	driveID ulid.ULID,
	rootNodeID uuid.UUID,
	createdAt, updatedAt time.Time,
) *Superblock {
	return &Superblock{
		id:         id,
		driveID:    driveID,
		rootNodeID: rootNodeID,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// SuperOperation is the per-drive root inode lookup surface.
// Full CRUD lives on superop.Repository.
type SuperOperation interface {
	Create(ctx context.Context, sb *Superblock) error
	Stat(ctx context.Context, id uuid.UUID) (*Superblock, error)
	GetByDriveID(ctx context.Context, driveID ulid.ULID) (*Superblock, error)
	Purge(ctx context.Context, id ulid.ULID) error
}
