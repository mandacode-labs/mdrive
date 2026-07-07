package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Create is a stub — should delegate to Repository.Create.
func (s *superOperation) Create(ctx context.Context, sb *fs.Superblock) error {
	panic("unimplemented")
}
