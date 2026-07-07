package superop

import (
	"context"

	"github.com/oklog/ulid/v2"
)

// Purge is a stub — should remove the superblock and all its
// data.
func (s *superOperation) Purge(ctx context.Context, id ulid.ULID) error {
	panic("unimplemented")
}
