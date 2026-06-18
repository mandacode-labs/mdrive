package drive

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the data-access contract for drives.
type Repository interface {
	Create(ctx context.Context, d *Drive, s *Storage) error
	GetByID(ctx context.Context, id string) (*Drive, error)
	GetByPublicID(ctx context.Context, publicID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
	Update(ctx context.Context, d *Drive) (*Drive, error)
	SoftDelete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	FindByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	FindDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
	FindDeletedByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	WithTx(ctx context.Context, fn func(Repository) error) error
}

// Exister checks whether an entity exists. Used to verify owner existence
// without coupling to the user package.
type Exister interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// RootCreator creates the root directory node for a drive.
// Implemented by the node.Service in the application layer (or by a stub
// during testing) to avoid circular dependencies.
type RootCreator interface {
	NewRootDirectory(ctx context.Context) (uuid.UUID, error)
}
