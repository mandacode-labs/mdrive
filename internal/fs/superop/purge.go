package superop

import (
	"context"

	"github.com/oklog/ulid/v2"
)

// Purge removes the superblock by drive id.
func (s *superOperation) Purge(ctx context.Context, id ulid.ULID) error {
	sb, err := s.repo.ReadByDriveID(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, sb.ID())
}