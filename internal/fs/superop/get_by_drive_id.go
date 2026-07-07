package superop

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/fs"
)

// GetByDriveID returns the superblock for a given drive.
func (s *superOperation) GetByDriveID(ctx context.Context, driveID ulid.ULID) (*fs.Superblock, error) {
	return s.repo.ReadByDriveID(ctx, driveID)
}
