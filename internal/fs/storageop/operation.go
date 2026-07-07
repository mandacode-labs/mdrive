package storageop

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// storageOperation is the ent-backed impl of fs.StorageOperation.
type storageOperation struct {
	repo      StorageRepository
	decryptor crypto.Decryptor // may be nil if no storage has encrypted keys
}

// NewStorageOperation wires the canonical impl. decryptor
// may be nil — only used when a storage row has an
// encrypted_secret_key to decrypt.
func NewStorageOperation(repo StorageRepository, decryptor crypto.Decryptor) fs.StorageOperation {
	return &storageOperation{repo: repo, decryptor: decryptor}
}

// GetBySuperblock returns the storage bound to a superblock.
// Returns nil + nil if no config exists (caller falls back
// to default IRSA).
func (s *storageOperation) GetBySuperblock(ctx context.Context, superblockID uuid.UUID) (*fs.Storage, error) {
	return s.repo.GetBySuperblock(ctx, superblockID)
}