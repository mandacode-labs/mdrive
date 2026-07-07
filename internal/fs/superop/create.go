package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Create persists a new superblock.
func (s *superOperation) Create(ctx context.Context, sb *fs.Superblock) error {
	return s.repo.Create(ctx, sb)
}
