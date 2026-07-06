package superop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Stat implements [vfs.SuperOperation].
func (s *superOperation) Stat(ctx context.Context, id uuid.UUID) (*vfs.Superblock, error) {
	panic("unimplemented")
}

