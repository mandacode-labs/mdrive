package superop

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// superOperation is the ent-backed impl of fs.SuperOperation.
type superOperation struct {
	repo Repository
	tm   entx.TxManager
}

// NewSuperblockOperation wires the canonical impl.
func NewSuperblockOperation(repo Repository, tm entx.TxManager) fs.SuperOperation {
	return &superOperation{repo: repo, tm: tm}
}

// Create persists a new superblock.
func (s *superOperation) Create(ctx context.Context, sb *fs.Superblock) error {
	return s.repo.Create(ctx, sb)
}

// Stat returns the superblock by id.
func (s *superOperation) Stat(ctx context.Context, id uuid.UUID) (*fs.Superblock, error) {
	return s.repo.Read(ctx, id)
}

// GetByDriveID returns the superblock for a given drive.
func (s *superOperation) GetByDriveID(ctx context.Context, driveID ulid.ULID) (*fs.Superblock, error) {
	return s.repo.ReadByDriveID(ctx, driveID)
}

// Purge removes the superblock by drive id.
func (s *superOperation) Purge(ctx context.Context, id ulid.ULID) error {
	sb, err := s.repo.ReadByDriveID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, sb.ID())
}