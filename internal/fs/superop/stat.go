package superop

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Stat returns the superblock by id.
func (s *superOperation) Stat(ctx context.Context, id uuid.UUID) (*fs.Superblock, error) {
	return s.repo.Read(ctx, id)
}
