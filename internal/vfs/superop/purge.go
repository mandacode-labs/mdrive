package superop

import (
	"context"

	"github.com/oklog/ulid/v2"
)

// Purge implements [vfs.SuperOperation].
func (s *superOperation) Purge(ctx context.Context, id ulid.ULID) error {
	panic("unimplemented")
}
