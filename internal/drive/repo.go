package drive

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

// Repository is the data-access contract for drives. Implemented
// by ent-backed repos; mocked in tests.
type Repository interface {
	Create(ctx context.Context, d *Drive) error
	Read(ctx context.Context, id ulid.ULID) (*Drive, error)
	UpdateFields(ctx context.Context, id ulid.ULID, name string, description *string) (*Drive, error)
	SoftDelete(ctx context.Context, id ulid.ULID, at time.Time) error
	Restore(ctx context.Context, id ulid.ULID) error
	Destroy(ctx context.Context, id ulid.ULID) error
	ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
	ReadStorage(ctx context.Context, driveID string) (*Storage, error)
	CreateStorage(ctx context.Context, s *Storage) error
}