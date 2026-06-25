package drive

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/user"
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

// Exister checks whether a user exists. drive uses this to verify
// the owner before creating the drive row. The interface lives
// in the user package; drive depends on it.
type Exister = user.Exister

// RootDirectoryCreator creates the root directory node for a
// drive. Implemented by the node.Service in the application layer
// (or by a stub during testing) to avoid circular dependencies.
type RootDirectoryCreator interface {
	CreateRootDirectory(ctx context.Context) (uuid.UUID, error)
}
