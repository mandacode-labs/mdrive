package drive

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Admin is the full handler-facing surface: create, update,
// soft-delete, restore, list-by-owner, list-deleted (admin),
// plus the read paths.
type Admin interface {
	Reader
	Create(ctx context.Context, actorID string, name, description string, cfg StorageConfig) (*Drive, uuid.UUID, error)
	Update(ctx context.Context, id string, name, description string) (*Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*Drive, error)
	ListByOwner(ctx context.Context, actorID string) ([]*Drive, error)
	ListDeletedForAdmin(ctx context.Context, isAdmin bool, before time.Time, limit int) ([]*Drive, error)
}

var _ Admin = (*Service)(nil)
