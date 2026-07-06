package superop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Stat is a stub — should delegate to Repository.Read.
func (s *superOperation) Stat(ctx context.Context, id uuid.UUID) (*vfs.Superblock, error) {
	panic("unimplemented")
}
