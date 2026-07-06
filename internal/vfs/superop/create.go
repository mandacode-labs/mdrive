package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Create implements [vfs.SuperOperation].
func (s *superOperation) Create(ctx context.Context, sb *vfs.Superblock) error {
	panic("unimplemented")
}
