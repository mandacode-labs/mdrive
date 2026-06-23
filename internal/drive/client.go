package drive

import (
	"context"
	"time"

	"github.com/google/uuid"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
)

// Client is the data-access contract the drive service needs. The
// core drive.Service satisfies it; tests may pass fakes.
type Client interface {
	Create(ctx context.Context, name string, desc *string, ownerID string, cfg coredrive.StorageConfig) (*coredrive.Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*coredrive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*coredrive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*coredrive.Storage, error)
	Update(ctx context.Context, id string, name, description *string) (*coredrive.Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*coredrive.Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*coredrive.Drive, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*coredrive.Drive, error)
}
