package vfs

import (
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Superblock is the per-drive inode carrier. Mirrors Linux
// struct super_block: holds the root inode id and a back-ref
// to its drive.
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

// NewSuperblock constructs a fresh Superblock with a new id.
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
	createdAt time.Time,
	updatedAt time.Time,
) *Superblock {
	return &Superblock{
		id:         id,
		driveID:    driveID,
		rootNodeID: rootNodeID,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}
