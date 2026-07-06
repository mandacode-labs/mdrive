package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Create is a stub — it should delegate to Repository.Create.
func (s *superOperation) Create(ctx context.Context, sb *vfs.Superblock) error {
	panic("unimplemented")
}
