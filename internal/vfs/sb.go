package vfs

import (
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Superblock is the filesystem root inode carrier. Mirrors
// Linux struct super_block: holds the root inode id and a
// back-reference to its drive. The drive_id is surfaced as
// ulid (matching Drive.ID()) so vfs can join without going
// through ent.
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

// NewSuperblock constructs a fresh Superblock. The id is
// generated; the caller provides rootNodeID and driveID.
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

// HydrateSuperblock reconstructs a Superblock from its
// persisted fields. Used by the ent-backed repository on load.
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