package storageop

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/fs"
)

// storageOperation is the ent-backed impl of fs.StorageOperation.
type storageOperation struct {
	repo StorageRepository
}

// NewStorageOperation wires the canonical impl.
func NewStorageOperation(repo StorageRepository) fs.StorageOperation {
	return &storageOperation{repo: repo}
}

// GetBySuperblock returns the storage bound to a superblock.
// Returns nil + nil if no config exists (caller falls back
// to default IRSA).
func (s *storageOperation) GetBySuperblock(ctx context.Context, superblockID uuid.UUID) (*fs.Storage, error) {
	return s.repo.GetBySuperblock(ctx, superblockID)
}