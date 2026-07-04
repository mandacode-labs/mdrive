package drive

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Reader is the read-only surface of a drive service. vfs uses
// it to resolve drive root IDs and storage config; permission
// checks are the caller's responsibility.
type Reader interface {
	GetByID(ctx context.Context, id string) (*Drive, error)
	GetByPublicID(ctx context.Context, publicID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
}

// StorageLookup is the storage-config-only read surface. The
// upload flow needs bucket/region/etc. but not the drive record
// itself.
type StorageLookup interface {
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
}

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

var (
	_ Reader        = (*Service)(nil)
	_ StorageLookup = (*Service)(nil)
	_ Admin         = (*Service)(nil)
)
